package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/deltron-fr/govisor/internal/config"
	"github.com/deltron-fr/govisor/internal/process"
)

type Supervisor struct {
	processes map[string]*process.Process
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

func NewSupervisor() *Supervisor {
	return &Supervisor{
		processes: make(map[string]*process.Process),
	}
}

func (s *Supervisor) Apply(pfInfo config.ConfigFile) error {
	runtimes := make([]*process.Process, 0, len(pfInfo.Processes))

	s.mu.Lock()
	for _, procConfig := range pfInfo.Processes {
		runtime := &process.Process{
			Config: procConfig,
			Status: process.StatusStarting,
		}
		s.processes[procConfig.Name] = runtime
		runtimes = append(runtimes, runtime)
	}
	s.mu.Unlock()

	go s.runProcesses(runtimes)

	return nil
}

func (s *Supervisor) runProcesses(runtimes []*process.Process) {
	for _, proc := range runtimes {
		s.wg.Add(1)
		proc.Status = process.StatusStarting
		proc.CreatedAt = time.Now()
		proc.UpdatedAt = time.Now()

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

		go func(proc *process.Process, procLog *os.File) {
			s.runProcess(proc, procLog)
		}(proc, procLog)
	}

	s.wg.Wait()
}

func (s *Supervisor) runProcess(proc *process.Process, procLog *os.File) {
	defer s.wg.Done()
	defer procLog.Close()

	backoff := time.Second

	for {
		cmd := exec.Command("sh", "-c", proc.Config.Command)
		cmd.Stdout = procLog
		cmd.Stderr = procLog

		err := cmd.Start()
		if err != nil {
			if proc.Config.OnRestart == "" || proc.Config.OnRestart == "no" {
				updateStatus(s, process.StatusCrashed, proc)
				break
			}

			updateStatus(s, process.StatusRestarting, proc)

			time.Sleep(backoff)
			backoff *= 2

			fmt.Fprintf(procLog, "Failed to start process %s: %v\n", proc.Config.Name, err)
			continue
		}

		backoff = time.Second

		updateStatus(s, process.StatusRunning, proc)

		err = cmd.Wait()
		if err != nil {
			fmt.Fprintf(procLog, "Process %s exited with error: %v\n", proc.Config.Name, err)

			if proc.Config.OnRestart == "" || proc.Config.OnRestart == "no" {
				updateStatus(s, process.StatusCrashed, proc)
				break
			}

			s.mu.Lock()
			proc.Status = process.StatusRestarting
			proc.UpdatedAt = time.Now()
			s.mu.Unlock()

			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		if proc.Config.OnRestart == "on_failure" {
			updateStatus(s, process.StatusStopped, proc)
			break
		}
	}
}

func (s *Supervisor) Snapshots() []process.Snapshot {
	s.mu.RLock()
	snapshots := make([]process.Snapshot, 0, len(s.processes))
	for _, proc := range s.processes {
		snapshots = append(snapshots, process.Snapshot{
			Name:      proc.Config.Name,
			Command:   proc.Config.Command,
			Status:    proc.Status,
			CreatedAt: proc.CreatedAt,
			UpdatedAt: proc.UpdatedAt,
		})
	}
	s.mu.RUnlock()

	return snapshots
}

func updateStatus[T process.ProcessStatus](supervisor *Supervisor, status T, proc *process.Process) {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()

	proc.Status = process.ProcessStatus(status)
	proc.UpdatedAt = time.Now()
}
