package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func applyHandler(configPath string) error {
	client := NewUnixClient("/tmp/govisor.sock")

	f, err := os.OpenFile(configPath, os.O_RDONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequest(http.MethodPut, "http://localhost/apply", f)
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
