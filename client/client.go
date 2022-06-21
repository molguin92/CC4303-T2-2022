/*
Copyright © 2022 Manuel Olguín Muñoz <manuel@olguinmunoz.xyz>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	tries     uint8
	maxTries  uint8
}

// Connect a client to the server, execute the proper handshake, then return the client.
func Connect(addr *net.UDPAddr, timeoutMs uint32, dgramSize uint32, maxRetries uint8) (*Client, error) {
	_, _ = fmt.Fprintf(
		os.Stderr,
		"Connecting to server %s (timeout %d ms, datagram size %d bytes)\n",
		addr,
		timeoutMs,
		dgramSize,
	)

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, err
	}

	// "connected"
	// execute handshake
	var seq uint8 = 0
	var actualSize uint32
	var actualTimeout uint32
	var serverSeq uint8
	var i uint8

	msg := []byte(fmt.Sprintf(handshakeFmt, seq, dgramSize, timeoutMs))

	for i = 0; i < maxRetries; i++ {
		_, err = conn.Write(msg)
		if err != nil {
			return nil, err
		}

		_ = conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))

		respBuf := make([]byte, len(msg))
		_, err := conn.Read(respBuf)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				// deadline, retry handshake
				continue
			} else {
				return nil, err
			}
		}

		// parse the response
		_, err = fmt.Sscanf(string(respBuf), respHandshakeFmt, &serverSeq, &actualSize, &actualTimeout)
		if err != nil {
			return nil, err
		} else if serverSeq != seq {
			return nil, fmt.Errorf(
				"sequence number mismatch during handshake. Expected: %d; received: %d", seq, serverSeq,
			)
		}
		//_, _ = fmt.Fprintln(os.Stdout, "Handshake successful!")
		//_, _ = fmt.Printf("Actual timeout: %d ms | Actual datagram size: %d bytes\n", actualTimeout, actualSize)
		_, _ = fmt.Fprintf(
			os.Stderr,
			"Set actual timeout to %d ms; actual datagram size to %d bytes\n",
			actualTimeout,
			actualSize,
		)

		// build the client and return
		return &Client{
			conn:      conn,
			seq:       seq,
			timeoutMs: actualTimeout,
			dgramSize: actualSize,
			tries:     0,
			maxTries:  maxRetries,
		}, nil
	}

	return nil, fmt.Errorf("too many retries (%d)", i)
}

func (client *Client) Close() {
	err := client.conn.Close()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Encountered error while attempting to close socket; silently ignoring...")
	}
}

// SendFile sends a file to the connected server
func (client *Client) SendFile(filePath string, rttCallback func(int, time.Duration)) (int, int, error) {
	_, _ = fmt.Fprintf(os.Stderr, "Sending file %s\n", filePath)
	fp, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
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
		return 0, 0, err
	}
	fileSize := fi.Size()
	_, _ = fmt.Fprintf(os.Stderr, "\rSent 0/%d bytes", fileSize)

	sentChunks := 0
	recvAcks := 0
	calcDropped := func() int {
		dropped := sentChunks - recvAcks
		if dropped > 0 {
			return dropped
		} else {
			return 0
		}
	}

	ti := time.Now()
	for {
		// read from file
		actualRead, err := fp.Read(chunk)
		reachedEOF = errors.Is(err, io.EOF) || (actualRead == 0)

		if !reachedEOF && (err != nil) {
			return fileSent, calcDropped(), err
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
		tSend := time.Now()
		for {
			client.tries++
			if client.tries > client.maxTries {
				return fileSent, calcDropped(), fmt.Errorf("too many retries (%d)", client.tries)
			}

			_, err = client.conn.Write(dgram[:actualRead+2])
			if err != nil {
				return fileSent, calcDropped(), err
			}
			sentChunks++

			_ = client.conn.SetReadDeadline(
				time.Now().Add(time.Duration(client.timeoutMs) * time.Millisecond),
			)

			_, err := client.conn.Read(ack)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					// timed out, resend chunk
					continue
				} else {
					return fileSent, calcDropped(), err
				}
			}

			// parse ack
			_, err = fmt.Sscanf(string(ack), ackFmt, &ackSeq)
			if err != nil {
				// could not parse ack, discard
				continue
			}
			// got ack
			recvAcks++

			if ackSeq == client.seq {
				// correct ack, move on to next chunk
				fileSent += actualRead
				client.tries = 0
				// report progress!
				_, _ = fmt.Fprintf(os.Stderr, "\rSent %d/%d bytes", fileSent, fileSize)
				break
			}
		}
		rttCallback(actualRead+2, time.Now().Sub(tSend))

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

	_, _ = fmt.Fprintf(os.Stderr, "File %s successfully sent!\n", filePath)
	_, _ = fmt.Fprintf(os.Stderr, "Sent %d bytes in %.03f seconds (%.03f MBps)\n", fileSent, dt, rate/10e3)
	return fileSent, calcDropped(), nil
}

func (client *Client) ReceiveFile(outPath string, rttCallback func(int, time.Duration)) (int, int, error) {
	_, _ = fmt.Fprintf(os.Stderr, "Receiving data and writing it to %s\n", outPath)

	// open file for writing
	fp, err := os.Create(outPath)
	if err != nil {
		return 0, 0, err
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
	_, _ = fmt.Fprintf(os.Stderr, "\rReceived %d bytes", fileRead)

	acksSent := 0
	chunksReceived := 0
	calcDropped := func() int {
		dropped := acksSent - chunksReceived
		if dropped > 0 {
			return dropped
		} else {
			return 0
		}
	}

	ti := time.Now()
	tSend := time.Now()

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
				_, err = client.conn.Write(prevAck)
				if err != nil {
					return fileRead, calcDropped(), err
				}
				acksSent++
				continue
			} else {
				return fileRead, calcDropped(), err
			}
		}

		// got a dgram, parse header to make sure it actually is data
		_, err = fmt.Sscanf(string(dgram[:2]), dataFmt, &serverSeq)
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
				_, err = client.conn.Write(prevAck)
				if err != nil {
					return fileRead, calcDropped(), err
				}
				acksSent++
				continue
			} else {
				// EOF and correct server seq
				// send ACK then break the loop and exit
				chunksReceived++
				_, err = client.conn.Write(ack)
				if err != nil {
					return fileRead, calcDropped(), err
				}
				acksSent++
				//totalRead += received
				client.seq = (client.seq + 1) % 2
				break
			}
		}

		// it's data, but is it the sequence we expected??
		if serverSeq != client.seq {
			// not the correct seq, resend previous ack
			_, err = client.conn.Write(prevAck)
			if err != nil {
				return fileRead, calcDropped(), err
			}
			acksSent++
			continue
		}

		// correct seq!
		chunksReceived++
		rttCallback(received, time.Now().Sub(tSend))

		// save data, then ack and update local seqs and acks
		_, err = fp.Write(dgram[2:received])
		if err != nil {
			return fileRead, calcDropped(), err
		}

		tSend = time.Now()
		_, err = client.conn.Write(ack)
		if err != nil {
			return fileRead, calcDropped(), err
		}
		acksSent++

		//totalRead += received
		fileRead += received - 2
		// progress!
		_, _ = fmt.Fprintf(os.Stderr, "\rReceived %d bytes", fileRead)

		// update seq and acks
		client.seq = (client.seq + 1) % 2

		copy(prevAck, ack)
		copy(ack, fmt.Sprintf(ackFmt, client.seq))
	}
	dt := time.Now().Sub(ti).Seconds()
	rate := float64(fileRead) / dt

	// reset conn deadline
	_ = client.conn.SetReadDeadline(time.Time{})

	_, _ = fmt.Fprintf(os.Stderr, "\nRead %d bytes in %.03f seconds (%.03f MBps)\n", fileRead, dt, rate/10e3)
	return fileRead, calcDropped(), nil
}
