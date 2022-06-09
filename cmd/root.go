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
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "CC4303_T2_2022",
	Short: "",
	Long:  "",
	Args:  cobra.ExactArgs(6),
	Run: func(cmd *cobra.Command, args []string) {
		// arguments
		timeoutMs, _ := strconv.ParseUint(args[0], 10, 32)
		dgramSize, _ := strconv.ParseUint(args[1], 10, 32)
		fileIn := args[2]
		//fileOut := args[3]

		address := args[4]
		port, _ := strconv.Atoi(args[5])
		fullAddress := fmt.Sprintf("%s:%d", address, port)

		// open socket
		//conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", address, port))
		udpAddr, err := net.ResolveUDPAddr("udp", fullAddress)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not resolve address %s\n", fullAddress)
			os.Exit(1)
		}

		c := client.Connect(udpAddr, uint32(timeoutMs), uint32(dgramSize))
		defer c.Close()
		c.SendFile(fileIn)
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
