package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func statusHandler(socketPath string) error {
	client := NewUnixClient(socketPath)

	req, err := http.NewRequest(http.MethodGet, "http://localhost/status", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	_, err = io.Copy(os.Stdout, res.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	return nil
}
