package client

import (
	"encoding/csv"
	"os"
)

func StartWriterThread(filePath string) chan [2]string {
	// open the channel and start the goroutine
	ioChan := make(chan [2]string)
	go writeLoop(ioChan, filePath)
	return ioChan
}

func writeLoop(ioChan chan [2]string, filePath string) {
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
