package config

type ConfigFile struct {
	Name      string          `yaml:"name"`
	Processes []ProcessConfig `yaml:"processes"`
}

type ProcessConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Command     string `yaml:"command"`
	OnRestart   string `yaml:"on_restart,omitempty"` // supports: no, always, on-failure, and unless-stopped
}

type OnRestart string

const (
	No OnRestart = "no"
	Always
	OnFailure
	UnlessStopped
)
