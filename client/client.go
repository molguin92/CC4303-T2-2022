package client

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const handshakeFmt = "C%1d%05d%05d"
const respHandshakeFmt = "A%1d%05d%05d"
const ackFmt = "A%1d"
const dataFmt = "D%1d"
const eofFmt = "E%1d"

type Client struct {
	conn      *net.UDPConn
	timeoutMs uint32
	dgramSize uint32
	seq       uint8
}

func writeToConn(conn net.Conn, data []byte) int {
	written, err := conn.Write(data)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Could not write data to socket!")
		panic(err)
	}
	return written
}

func readFromConn(conn net.Conn, recvBuf []byte) int {
	read, err := conn.Read(recvBuf)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Could not read from socket!")
		panic(err)
	}
	return read
}

func parseMessage(msg string, formatStr string, vars ...any) {
	_, err := fmt.Sscanf(msg, formatStr, vars...)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Could not parse message!")
		_, _ = fmt.Fprintf(os.Stderr, "Message: %s\n", msg)
		panic(err)
	}
}

// Connect a client to the server, execute the proper handshake, then return the client.
func Connect(addr *net.UDPAddr, timeoutMs uint32, dgramSize uint32) *Client {
	_, _ = fmt.Fprintf(os.Stdout, "Connecting to server %s\n", addr)
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Could not connect!")
		panic(err)
	}
	_, _ = fmt.Fprintln(os.Stdout, "Connection successful, attempting handshake...")
	_, _ = fmt.Printf("Suggested timeout: %d ms | Suggested datagram size: %d bytes\n", timeoutMs, dgramSize)

	// "connected"
	// execute handshake
	var seq uint8 = 0

	msg := []byte(fmt.Sprintf(handshakeFmt, seq, dgramSize, timeoutMs))
	_ = writeToConn(conn, msg)

	respBuf := make([]byte, len(msg))
	_ = readFromConn(conn, respBuf)

	// parse the response
	var actualSize uint32
	var actualTimeout uint32
	var serverSeq uint8

	parseMessage(string(respBuf), respHandshakeFmt, &serverSeq, &actualSize, &actualTimeout)
	if serverSeq != seq {
		_, _ = fmt.Fprintln(os.Stderr, "Seq mismatch!")
		_, _ = fmt.Fprintf(os.Stderr, "Local seq: %d\nServer seq: %d\n", seq, serverSeq)
		panic(serverSeq)
	}
	_, _ = fmt.Fprintln(os.Stdout, "Handshake successful!")
	_, _ = fmt.Printf("Actual timeout: %d ms | Actual datagram size: %d bytes\n", actualTimeout, actualSize)

	// build the client and return
	return &Client{
		conn:      conn,
		seq:       seq,
		timeoutMs: actualTimeout,
		dgramSize: actualSize,
	}
}

func (client *Client) Close() {
	err := client.conn.Close()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Encountered error while attempting to close socket; silently ignoring...")
	}
}

// SendFile sends a file to the connected server
func (client *Client) SendFile(filePath string) {
	_, _ = fmt.Printf("Sending file %s\n", filePath)
	fp, err := os.Open(filePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Could not open file %s\n", filePath)
		panic(err)
	}
	defer func(fp *os.File) {
		_ = fp.Close()
	}(fp)

	// send + ack loop
	fileSent := 0
	//totalSent := 0
	chunk := make([]byte, client.dgramSize-2)
	dgram := make([]byte, client.dgramSize)
	ack := make([]byte, 2)
	var ackSeq uint8
	reachedEOF := false

	// file size for progress
	fi, err := fp.Stat()
	if err != nil {
		panic(err)
	}
	fileSize := fi.Size()
	_, _ = fmt.Printf("\rSent 0/%d bytes", fileSize)

	ti := time.Now()
	for {
		// read from file
		actualRead, err := fp.Read(chunk)
		reachedEOF = errors.Is(err, io.EOF) || (actualRead == 0)

		if !reachedEOF && (err != nil) {
			_, _ = fmt.Fprintf(os.Stderr, "Error reading from source file: %s\n", err)
			panic(err)
		}

		// increment seq if sending a chunk
		client.seq = (client.seq + 1) % 2

		// handle EOF
		if reachedEOF {
			copy(dgram, fmt.Sprintf(eofFmt, client.seq))
			actualRead = 0
		} else {
			copy(dgram, fmt.Sprintf(dataFmt, client.seq))
			copy(dgram[2:], chunk[:actualRead])
		}

		// send chunk or EOF
		for {
			writeToConn(client.conn, dgram[:actualRead+2])
			_ = client.conn.SetReadDeadline(
				time.Now().Add(time.Duration(client.timeoutMs) * time.Millisecond),
			)

			//totalSent += actualRead + 2

			_, err := client.conn.Read(ack)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					// timed out, resend chunk
					continue
				} else {
					_, _ = fmt.Fprintln(os.Stderr, "Could not send file chunk!")
					panic(err)
				}
			}

			// parse ack
			_, err = fmt.Sscanf(string(ack), ackFmt, &ackSeq)
			if err != nil {
				// could not parse ack, discard
				continue
			}

			if ackSeq == client.seq {
				// correct ack, move on to next chunk
				fileSent += actualRead
				// report progress!
				_, _ = fmt.Printf("\rSent %d/%d bytes", fileSent, fileSize)
				break
			}
		}

		if reachedEOF {
			// sent EOF and got ack for EOF
			_, _ = fmt.Println()
			break
		}
	}
	dt := time.Now().Sub(ti).Seconds()
	rate := float64(fileSent) / dt

	// reset conn deadline
	_ = client.conn.SetReadDeadline(time.Time{})

	_, _ = fmt.Printf("Succesfully sent file %s!\n", filePath)
	_, _ = fmt.Printf("Sent %d bytes in %.03f seconds (%.03f MBps)\n", fileSent, dt, rate/10e3)
}

func (client *Client) ReceiveFile(outPath string) {
	_, _ = fmt.Println("Receiving data from remote...")
	_, _ = fmt.Printf("Writing output to %s\n", outPath)

	// open file for writing
	fp, err := os.Create(outPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Could not open file %s for writing!\n", outPath)
		panic(err)
	}
	defer func(fp *os.File) {
		_ = fp.Close()
	}(fp)

	// preallocate space for data
	dgram := make([]byte, client.dgramSize)
	var serverSeq uint8

	// prepare acks for sending
	// reset client seq
	client.seq = 0
	prevAck := []byte(fmt.Sprintf(ackFmt, 1))
	ack := []byte(fmt.Sprintf(ackFmt, client.seq))

	fileRead := 0
	//totalRead := 0
	_, _ = fmt.Printf("\rReceived %d bytes", fileRead)

	ti := time.Now()
	for {
		// set timeout!
		_ = client.conn.SetReadDeadline(
			time.Now().Add(time.Duration(client.timeoutMs) * time.Millisecond),
		)

		// receive a chunk
		received, err := client.conn.Read(dgram)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				// timed out waiting for chunk, resend previous ACK
				writeToConn(client.conn, prevAck)
				continue
			} else {
				_, _ = fmt.Fprintln(os.Stderr, "Could not read from socket!")
				panic(err)
			}
		}

		// got a dgram, parse header to make sure it actually is data
		_, err = fmt.Sscanf(string(dgram[:2]), dataFmt, &serverSeq)
		fmt.Println(string(dgram[:2]))
		if err != nil {
			// could not parse header
			// it could be an EOF header
			_, err = fmt.Sscanf(string(dgram[:2]), eofFmt, &serverSeq)
			if err != nil {
				// could not parse, probably stray ack
				// ignore
				continue
			} else if serverSeq != client.seq {
				// EOF and received seq doesn't match expected seq
				// resend ACK and retry
				writeToConn(client.conn, prevAck)
				continue
			} else {
				// EOF and correct server seq
				// send ACK then break the loop and exit
				writeToConn(client.conn, ack)
				//totalRead += received
				client.seq = (client.seq + 1) % 2
				break
			}
		}

		// it's data, but is it the sequence we expected??
		if serverSeq != client.seq {
			// not the correct seq, resend previous ack
			writeToConn(client.conn, prevAck)
			continue
		}

		// correct seq!
		// save data, then ack and update local seqs and acks
		_, err = fp.Write(dgram[2:received])
		if err != nil {
			_, _ = fmt.Fprintf(
				os.Stderr,
				"Could not write data chunk (%d bytes) to file!",
				received-2,
			)
			panic(err)
		}

		writeToConn(client.conn, ack)

		//totalRead += received
		fileRead += received - 2
		// progress!
		_, _ = fmt.Printf("\rReceived %d bytes", fileRead)

		// update seq and acks
		client.seq = (client.seq + 1) % 2

		copy(prevAck, ack)
		copy(ack, fmt.Sprintf(ackFmt, client.seq))
	}
	dt := time.Now().Sub(ti).Seconds()
	rate := float64(fileRead) / dt

	// reset conn deadline
	_ = client.conn.SetReadDeadline(time.Time{})

	//_, _ = fmt.Printf("Succesfully received %d bytes of data into %s!\n", fileRead, outPath)
	_, _ = fmt.Printf("\nRead %d bytes in %.03f seconds (%.03f MBps)\n", fileRead, dt, rate/10e3)

}
