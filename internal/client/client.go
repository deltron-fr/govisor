package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const baseURL = "http://localhost"

type Client struct {
	httpClient *http.Client
}

func New(socketPath string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
	}
}

func (c *Client) ApplyHandler(configPath string, writer io.Writer) error {
	f, err := os.OpenFile(configPath, os.O_RDONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequest(http.MethodPut, baseURL+"/apply", f)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(req, writer)
}

func (c *Client) StatusHandler(writer io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/status", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(req, os.Stdout)
}

func (c *Client) do(req *http.Request, out io.Writer) error {
	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("request failed with status %d and failed to read error body: %w", res.StatusCode, err)
		}

		trimmedBody := strings.TrimSpace(string(bytes.TrimSpace(body)))
		if trimmedBody == "" {
			return fmt.Errorf("request failed with status %d", res.StatusCode)
		}

		return fmt.Errorf("request failed with status %d: %s", res.StatusCode, trimmedBody)
	}

	if _, err := io.Copy(out, res.Body); err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return nil
}
