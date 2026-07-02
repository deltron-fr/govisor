package process

import (
	"os"
	"os/exec"
	"time"

	"github.com/deltron-fr/govisor/internal/config"
)

type ProcessStatus int

func (ps ProcessStatus) String() string {
	switch ps {
	case StatusStarting:
		return "STARTING"
	case StatusRunning:
		return "RUNNING"
	case StatusStopped:
		return "STOPPED"
	case StatusRestarting:
		return "RESTARTING"
	case StatusCrashed:
		return "CRASHED"
	default:
		return "UNKNOWN"
	}
}

const (
	StatusStarting ProcessStatus = iota
	StatusRunning
	StatusStopped
	StatusRestarting
	StatusCrashed
)

type Process struct {
	Cmd         *exec.Cmd
	LogFile     *os.File
	LogFileName string
	Config      config.ProcessConfig
	Status      ProcessStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Snapshot struct {
	Name      string
	Command   string
	Status    ProcessStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
