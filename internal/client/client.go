package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deltron-fr/govisor/internal/ipc"
	"github.com/deltron-fr/govisor/internal/server"
)

const baseURL = "http://localhost"

type Client struct {
	httpClient *http.Client
	socketPth  string
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

func (c *Client) ApplyHandler(writer io.Writer, configPath string) error {
	resolvedConfigPath, err := resolveConfigPath(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, baseURL+"/apply", strings.NewReader(resolvedConfigPath))
	if err != nil {
		return fmt.Errorf("failed to create apply request for config path %q: %w", resolvedConfigPath, err)
	}

	return c.do(req, writer)
}

func (c *Client) StatusHandler(writer io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/status", nil)
	if err != nil {
		return fmt.Errorf("failed to create status request: %w", err)
	}

	return c.do(req, writer)
}

func (c *Client) LogsHandler(writer io.Writer, name string) error {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/logs/"+name, nil)
	if err != nil {
		return fmt.Errorf("failed to create logs request for process %q: %w", name, err)
	}

	return c.do(req, writer)
}

func (c *Client) StartHandler(writer io.Writer) error {
	path, err := ipc.SocketPath()
	if err != nil {
		return fmt.Errorf("failed to configure socket path: %w", err)
	}

	server := server.New(path)
	server.Serve()
	return nil
}

func (c *Client) StopHandler(writer io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/stop", nil)
	if err != nil {
		return fmt.Errorf("failed to create stop request: %w", err)
	}

	return c.do(req, writer)
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

func resolveConfigPath(inputPath string) (string, error) {
	if filepath.IsAbs(inputPath) {
		return inputPath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(cwd, inputPath))
	if err != nil {
		return "", err
	}

	return resolved, nil
}
