package supervisor

import (
	"os/exec"
	"path/filepath"
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
