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
	_, _ = fmt.Fprintln(os.Stdout, "Handshake succesful!")

	// build the client and return
	return &Client{
		conn:      conn,
		seq:       (seq + 1) % 2,
		timeoutMs: actualTimeout,
		dgramSize: actualSize,
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
	chunk := make([]byte, client.dgramSize-2)
	dgram := make([]byte, client.dgramSize)
	ack := make([]byte, 2)
	var ackSeq uint8
	reachedEOF := false

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
			parseMessage(string(ack), ackFmt, &ackSeq)

			if ackSeq == client.seq {
				// correct ack, move on to next chunk
				break
			}
		}

		if reachedEOF {
			// sent EOF and got ack for EOF
			break
		}
	}
	_, _ = fmt.Printf("Succesfully sent file %s!\n", filePath)
}
