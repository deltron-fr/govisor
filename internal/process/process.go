package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"go.yaml.in/yaml/v4"
)

type ProcessConfigFileInfo struct {
	Name      string          `yaml:"name"`
	Processes []ProcessConfig `yaml:"processes"`
}

type ProcessConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Command     string `yaml:"command"`
}

type ProcessStatus int

const (
	StatusStarting ProcessStatus = iota
	StatusRunning
	StatusStopped
	StatusRestarting
	StatusCrashed
)

type Process struct {
	Cmd    *exec.Cmd
	Config ProcessConfig
	Status ProcessStatus
}

type Supervisor struct {
	processes map[string]*Process
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

func NewSupervisor() *Supervisor {
	return &Supervisor{
		processes: make(map[string]*Process),
	}
}

func (s *Supervisor) StartSupervisor(filePath string) error {
	pfInfo, err := s.parseProcessFile(filePath)
	if err != nil {
		return err
	}

	for _, procConfig := range pfInfo.Processes {
		s.processes[procConfig.Name] = &Process{
			Config: procConfig,
			Status: StatusStarting,
		}
	}

	s.startProcesses()

	return nil
}

func (s *Supervisor) parseProcessFile(filePath string) (*ProcessConfigFileInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var pcInfo ProcessConfigFileInfo

	if err := yaml.Unmarshal(data, &pcInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return &pcInfo, nil
}

func (s *Supervisor) startProcesses() {
	for _, proc := range s.processes {
		s.wg.Add(1)

		procLogFile := fmt.Sprintf("%s.log", proc.Config.Name)
		f, err := os.OpenFile(procLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error opening log file:", err)
			// TODO: kill all processes started so far and exit
			os.Exit(1)
		}

		procLog := f

		execCmd := exec.Command("sh", "-c", proc.Config.Command)
		execCmd.Stdout = procLog
		execCmd.Stderr = procLog

		go func(proc *Process, cmd *exec.Cmd) {
			defer s.wg.Done()
			defer f.Close()

			err := cmd.Start()
			if err != nil {
				s.mu.Lock()
				proc.Status = StatusCrashed
				s.mu.Unlock()

				fmt.Fprintf(procLog, "Failed to start process %s: %v\n", proc.Config.Name, err)
				return
			}

			s.mu.Lock()
			proc.Status = StatusRunning
			s.mu.Unlock()

			err = cmd.Wait()

			s.mu.Lock()
			defer s.mu.Unlock()

			if err != nil {
				// TODO: Implement process restart logic here
				fmt.Fprintf(procLog, "Process %s exited with error: %v\n", proc.Config.Name, err)
				proc.Status = StatusCrashed
				return
			}

			proc.Status = StatusStopped
		}(proc, execCmd)
	}

	s.wg.Wait()
}
