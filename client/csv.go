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
	"encoding/csv"
	"os"
	"sync"
)

func StartWriterThread(filePath string, waitGroup *sync.WaitGroup) chan [2]string {
	// open the channel and start the goroutine
	ioChan := make(chan [2]string)
	go writeLoop(ioChan, filePath, waitGroup)
	waitGroup.Add(1)
	return ioChan
}

func writeLoop(ioChan chan [2]string, filePath string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	fp, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}
	defer func(fp *os.File) {
		_ = fp.Close()
	}(fp)

	// read from channel and write to file
	w := csv.NewWriter(fp)

	for row := range ioChan {
		if err = w.Write(row[:]); err != nil {
			panic(err)
		}
	}

	// channel closed, flush
	w.Flush()
	if err = w.Error(); err != nil {
		panic(err)
	}
}
