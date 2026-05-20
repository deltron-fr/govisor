package main

import (
	"context"
	"net"
	"net/http"
	"time"
)

var SOCKET_PATH = "/tmp/govisor.sock"

func main() {
	Execute()
}

func NewUnixClient(socketPath string) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
}
