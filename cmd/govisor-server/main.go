package main

import (
	"github.com/deltron-fr/govisor/internal/ipc"
	"github.com/deltron-fr/govisor/internal/server"
)

func main() {
	server := server.New(ipc.DEFAULT_SOCKET_PATH)
	server.Serve()
}
