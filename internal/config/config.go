package config

import (
	"bytes"
	"fmt"
)

type ConfigFile struct {
	Name      string          `yaml:"name"`
	Processes []ProcessConfig `yaml:"processes"`
}

type ProcessConfig struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Command     string  `yaml:"command"`
	Restart     Restart `yaml:"restart,omitempty"` // supports: never, always, on-failure, and unless-stopped
}

type Restart int

const (
	Always Restart = iota
	Never
	OnFailure
	UnlessStopped
)

func (r Restart) String() string {
	switch r {
	case Never:
		return "never"
	case Always:
		return "always"
	case OnFailure:
		return "on_failure"
	case UnlessStopped:
		return "unless_stopped"
	default:
		return fmt.Sprintf("Restart(%d)", r)
	}
}

func (r Restart) MarshalText() ([]byte, error) {
	switch r {
	case Never, Always, OnFailure, UnlessStopped:
		return []byte(r.String()), nil
	default:
		return nil, fmt.Errorf("unknown restart policy state: %d", r)
	}
}

func (r *Restart) UnmarshalText(text []byte) error {
	cleaned := bytes.ToLower(bytes.TrimSpace(text))

	switch string(cleaned) {
	case "never":
		*r = Never
	case "always":
		*r = Always
	case "on_failure", "on-failure": // Handling an extra format variation
		*r = OnFailure
	case "unless_stopped", "unless-stopped":
		*r = UnlessStopped
	default:
		return fmt.Errorf("invalid restart policy: %q", string(text))
	}
	return nil
}
