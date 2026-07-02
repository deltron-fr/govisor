package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/deltron-fr/govisor/internal/process"
)

// Getters and Setters for working with private supervisor fields
func (s *Supervisor) SetConfigFilePath(path string) {
	s.configFilePath = path
}

func (s *Supervisor) SetLogFilePath(path string) {
	s.logFilePath = path
}

func (s *Supervisor) GetLogFilePath() string {
	return s.logFilePath
}

// configBaseDir returns the config file path(exculding the filename)
func (s *Supervisor) configBaseDir() string {
	if s.configFilePath == "" {
		return ""
	}

	return filepath.Dir(s.configFilePath)
}

func (s *Supervisor) GetProcess(name string) (*process.Process, error) {
	s.mu.Lock()
	process, ok := s.processes[name]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("process %s: does not exist", name)
	}

	return process, nil
}

func configureLogPath() string {
	stateHomeDir, exists := os.LookupEnv("XDG_STATE_HOME")
	if exists {
		if stateHomeDir != "" {
			fmt.Println("used xdg state home")
			return filepath.Join(stateHomeDir, "govisor/logs")
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".local/state/govisor/logs")
}

func hasPathSeparator(input string) bool {
	return filepath.Base(input) != input
}

func commandExistsInPath(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func resolvePath(path string, baseDir string) string {
	if filepath.IsAbs(path) || baseDir == "" {
		return filepath.Clean(path)
	}

	return filepath.Join(baseDir, path)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
