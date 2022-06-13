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
	"CC4303_T2_2022/client"
	"fmt"
	"github.com/spf13/cobra"
	"net"
	"os"
	"strconv"
	"time"
)

var RecordRTTs bool

const SendRTTFile = "./sendRTTs.csv"
const RecvRTTFile = "./recvRTTs.csv"

var RTTHeader = [2]string{"size_bytes", "rtt_seconds"}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "CC4303_T2_2022 TIMEOUT_MS DATAGRAM_SIZE_BYTES INPUT_FILE OUTPUT_FILE HOST PORT",
	Short: "",
	Long:  "",
	Args:  cobra.ExactArgs(6),
	Run: func(cmd *cobra.Command, args []string) {
		// arguments
		timeoutMs, _ := strconv.ParseUint(args[0], 10, 32)
		dgramSize, _ := strconv.ParseUint(args[1], 10, 32)
		fileIn := args[2]
		fileOut := args[3]

		address := args[4]
		port, _ := strconv.Atoi(args[5])
		fullAddress := fmt.Sprintf("%s:%d", address, port)

		udpAddr, err := net.ResolveUDPAddr("udp", fullAddress)
		if err != nil {
			panic(err)
		}

		c, err := client.Connect(udpAddr, uint32(timeoutMs), uint32(dgramSize), 10)
		if err != nil {
			panic(err)
		}
		defer c.Close()

		// "connected", start the CSV writing threads
		var sendChan chan [2]string
		var recvChan chan [2]string

		var sendCallback func(int, time.Duration)
		var recvCallback func(int, time.Duration)

		if RecordRTTs {
			// start writing threads
			sendChan = client.StartWriterThread(SendRTTFile)
			recvChan = client.StartWriterThread(RecvRTTFile)

			sendChan <- RTTHeader
			recvChan <- RTTHeader

			sendCallback = func(size int, rtt time.Duration) {
				sendChan <- [2]string{fmt.Sprintf("%d", size), fmt.Sprintf("%f", rtt.Seconds())}
			}

			recvCallback = func(size int, rtt time.Duration) {
				recvChan <- [2]string{fmt.Sprintf("%d", size), fmt.Sprintf("%f", rtt.Seconds())}
			}

		} else {
			sendChan = make(chan [2]string)
			recvChan = make(chan [2]string)

			sendCallback = func(i int, duration time.Duration) {
				// no-op
			}
			recvCallback = func(i int, duration time.Duration) {
				// no-op
			}
		}

		defer close(sendChan)
		defer close(recvChan)

		_, err = c.SendFile(fileIn, sendCallback)
		if err != nil {
			panic(err)
		}

		_, err = c.ReceiveFile(fileOut, recvCallback)
		if err != nil {
			panic(err)
		}
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

func init() {
	rootCmd.PersistentFlags().BoolVarP(
		&RecordRTTs,
		"record-rtts",
		"r",
		false,
		"Record RTTs; samples will be output as CSV files in the current directory.")
}
