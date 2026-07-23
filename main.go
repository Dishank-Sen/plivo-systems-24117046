package main

import (
	"fmt"
	"os"
)

func main() {
	// Determine if we're sender or receiver based on binary name
	progName := os.Args[0]

	// Check the last part of the path
	if len(progName) >= 6 && progName[len(progName)-6:] == "sender" {
		mainSender()
	} else if len(progName) >= 8 && progName[len(progName)-8:] == "receiver" {
		mainReceiver()
	} else {
		fmt.Println("Usage: build as ./sender or ./receiver")
		fmt.Println("This binary determines its role from its filename")
		os.Exit(1)
	}
}
