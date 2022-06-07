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

package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

const ClientHandshakeFmt = "C%1d%05d%05d"
const ServerHandshakeFmt = "A%1d%05d%05d"
const ACKFmt = "A%1d"
const DATAFmt = "D%1d"
const EOFFmt = "E%1d"

func sendLoop(conn *net.UDPConn, timeoutMs int, dgramSize int, fileIn *os.File) int {
	seq := 0

	msg := []byte(fmt.Sprintf(ClientHandshakeFmt, seq, dgramSize, timeoutMs))
	_, _ = conn.Write(msg)

	respBuf := make([]byte, len(msg))
	_, _ = conn.Read(respBuf)

	// parse the response
	var actualSize int
	var actualTimeout int
	var serverSeq int8

	fmt.Sscanf(string(respBuf), ServerHandshakeFmt, &serverSeq, &actualSize, &actualTimeout)

	// send + ack loop
	chunk := make([]byte, actualSize-2)
	dgram := make([]byte, actualSize)
	ack := make([]byte, 2)
	var ackSeq int
	reachedEOF := false

	for {
		// read from file
		actualRead, err := fileIn.Read(chunk)
		reachedEOF = errors.Is(err, io.EOF) || (actualRead == 0)

		if !reachedEOF && (err != nil) {
			fmt.Fprintf(os.Stderr, "Error reading from source file: %s\n", err)
			os.Exit(1)
		}

		// increment seq if sending a chunk
		seq = (seq + 1) % 2

		// handle EOF
		if reachedEOF {
			copy(dgram, fmt.Sprintf(EOFFmt, seq))
			actualRead = 0
		} else {
			copy(dgram, fmt.Sprintf(DATAFmt, seq))
			copy(dgram[2:], chunk[:actualRead])
		}

		// send chunk or EOF
		for {
			conn.Write(dgram[:actualRead+2])
			conn.SetReadDeadline(time.Now().Add(time.Duration(actualTimeout) * time.Millisecond))
			_, err := conn.Read(ack)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					// timed out, resend chunk
					continue
				} else {
					fmt.Fprintf(os.Stderr, "Could not send file chunk! Error: %s", err)
					os.Exit(1)
				}
			}

			// parse ack
			fmt.Sscanf(string(ack), ACKFmt, &ackSeq)

			if ackSeq == seq {
				// correct ack, move on to next chunk
				break
			}
		}

		if reachedEOF {
			// sent EOF and got ack for EOF
			break
		}
	}

	return seq
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "CC4303_T2_2022",
	Short: "",
	Long:  "",
	Args:  cobra.ExactArgs(6),
	Run: func(cmd *cobra.Command, args []string) {
		// arguments
		timeoutMs, _ := strconv.Atoi(args[0])
		dgramSize, _ := strconv.Atoi(args[1])
		fileIn := args[2]
		//fileOut := args[3]

		address := args[4]
		port, _ := strconv.Atoi(args[5])
		fullAddress := fmt.Sprintf("%s:%d", address, port)

		// open file: TODO
		fp, err := os.Open(fileIn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open file %s", fileIn)
			os.Exit(1)
		}
		defer fp.Close()

		// open socket
		//conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", address, port))
		udpAddr, err := net.ResolveUDPAddr("udp", fullAddress)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not resolve address %s\n", fullAddress)
			os.Exit(1)
		}

		conn, err := net.DialUDP("udp4", nil, udpAddr)
		if err != nil {
			fmt.Println("Error connecting!")
			os.Exit(1)
		}
		defer conn.Close()

		sendLoop(conn, timeoutMs, dgramSize, fp)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {}
