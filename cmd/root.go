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
	"encoding/json"
	"fmt"
	"github.com/molguin92/CC4303-T2-2022/client"
	"github.com/spf13/cobra"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

var RecordRTTs bool
var RecordStats bool

const SendRTTFile = "./sendRTTs.csv"
const RecvRTTFile = "./recvRTTs.csv"
const StatsFile = "./stats.json"

var RTTHeader = [2]string{"size_bytes", "rtt_seconds"}

type ClientStats struct {
	TimeSeconds    float64 `json:"TimeSeconds"`
	DroppedPackets int64   `json:"DroppedPackets"`
	DroppedAcks    int64   `json:"DroppedAcks"`
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "CC4303-T2-2022 TIMEOUT_MS DATAGRAM_SIZE_BYTES INPUT_FILE OUTPUT_FILE HOST PORT",
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
		var sendCallback func(int, time.Duration)
		var recvCallback func(int, time.Duration)

		if RecordRTTs {
			_, _ = fmt.Fprintf(os.Stderr,
				"Writing RTTs to %s and %s\n",
				SendRTTFile,
				RecvRTTFile,
			)

			// start writing threads
			var waitGroup sync.WaitGroup
			sendChan := client.StartWriterThread(SendRTTFile, &waitGroup)
			recvChan := client.StartWriterThread(RecvRTTFile, &waitGroup)

			defer func() {
				_, _ = fmt.Fprintf(os.Stderr,
					"RTT records have been output to %s and %s\n",
					SendRTTFile,
					RecvRTTFile,
				)
				close(sendChan)
				close(recvChan)
				waitGroup.Wait()
			}()

			sendChan <- RTTHeader
			recvChan <- RTTHeader

			sendCallback = func(size int, rtt time.Duration) {
				sendChan <- [2]string{fmt.Sprintf("%d", size), fmt.Sprintf("%f", rtt.Seconds())}
			}

			recvCallback = func(size int, rtt time.Duration) {
				recvChan <- [2]string{fmt.Sprintf("%d", size), fmt.Sprintf("%f", rtt.Seconds())}
			}

		} else {
			sendCallback = func(i int, duration time.Duration) {
				// no-op
			}
			recvCallback = func(i int, duration time.Duration) {
				// no-op
			}
		}

		ti := time.Now()
		_, droppedChunks, err := c.SendFile(fileIn, sendCallback)
		if err != nil {
			panic(err)
		}

		_, droppedAcks, err := c.ReceiveFile(fileOut, recvCallback)
		if err != nil {
			panic(err)
		}
		dt := time.Now().Sub(ti)

		_, _ = fmt.Fprintf(os.Stderr, "Dropped %d chunks and %d acks.\n", droppedChunks, droppedAcks)
		_, _ = fmt.Fprintf(os.Stderr, "Total time: %s\n", dt)

		if RecordStats {
			fp, err := os.Create(StatsFile)
			if err != nil {
				panic(err)
			}
			defer func(fp *os.File) {
				_ = fp.Close()
			}(fp)

			stats := ClientStats{
				TimeSeconds:    dt.Seconds(),
				DroppedPackets: int64(droppedChunks),
				DroppedAcks:    int64(droppedAcks),
			}
			statsB, err := json.Marshal(stats)
			if err != nil {
				panic(err)
			}
			_, _ = fmt.Fprintf(fp, "%s\n", statsB)
			_, _ = fmt.Fprintf(os.Stderr, "Stats have been output to %s\n", StatsFile)
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
		fmt.Sprintf(
			"Record RTTs; samples will be output as CSV files %s and %s in the current directory.",
			RecvRTTFile,
			SendRTTFile,
		),
	)

	rootCmd.PersistentFlags().BoolVarP(
		&RecordStats,
		"record-stats",
		"s",
		false,
		fmt.Sprintf(
			"Record stats for total time, dropped packets, and dropped ACKS. "+
				"Will be stored as a JSON file %s in the current directory.",
			StatsFile,
		),
	)
}
