package main

import (
	"github.com/deltron-fr/govisor/internal/client"
	"github.com/deltron-fr/govisor/internal/ipc"
)

var appClient = client.New(ipc.DEFAULT_SOCKET_PATH)

func main() {
	Execute()
}
