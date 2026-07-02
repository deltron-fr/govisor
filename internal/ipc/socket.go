package ipc

import (
	"os"
	"path/filepath"
)

func SocketPath() (string, error) {
	runtimeDir, exists := os.LookupEnv("XDG_RUNTIME_DIR")
	if exists {
		if runtimeDir != "" {
			return filepath.Join(runtimeDir, "govisor/govisor.sock"), nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".local/state/govisor/run/govisor.sock"), nil
}
