package config


type ConfigFile struct {
	Name      string          `yaml:"name"`
	Processes []ProcessConfig `yaml:"processes"`
}

type ProcessConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Command     string `yaml:"command"`
}

