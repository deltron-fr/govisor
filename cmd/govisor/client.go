package main

import (
	"fmt"
	"os"

	"github.com/deltron-fr/govisor/internal/client"
	"github.com/deltron-fr/govisor/internal/ipc"
)

func getSocketPath() string {
	path, err := ipc.SocketPath()
	if err != nil {
		fmt.Printf("couldn't get socket path: %v\n", err)
		os.Exit(1)
	}

	return path
}

var appClient = client.New(getSocketPath())

func main() {
	Execute()
}
