package supervisor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/deltron-fr/govisor/internal/config"
	"github.com/deltron-fr/govisor/internal/process"
)

type Supervisor struct {
	processes  map[string]*process.Process
	maxLogSize int
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

func NewSupervisor() *Supervisor {
	return &Supervisor{
		processes:  make(map[string]*process.Process),
		maxLogSize: 10 * (1 << 20),
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

	go s.runLogRotation()

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
			s.StopProcesses()
			os.Exit(1)
		}

		procLog := f
		proc.LogFile = procLog

		execCmd := exec.Command("sh", "-c", proc.Config.Command)
		execCmd.Stdout = procLog
		execCmd.Stderr = procLog
		proc.Cmd = execCmd

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
		cmd := exec.Command("sh", "-c", "exec "+proc.Config.Command)
		cmd.Stdout = procLog
		cmd.Stderr = procLog

		err := cmd.Start()
		if err != nil {
			if proc.Config.Restart == config.Never {
				updateStatus(s, process.StatusCrashed, proc)
				return
			}

			updateStatus(s, process.StatusRestarting, proc)

			time.Sleep(backoff)
			backoff *= 2
			if backoff >= 300*time.Second {
				backoff = 30 * time.Second
			}

			fmt.Fprintf(procLog, "Failed to start process %s: %v\n", proc.Config.Name, err)
			continue
		}

		backoff = time.Second

		updateStatus(s, process.StatusRunning, proc)

		err = cmd.Wait()
		if err != nil {
			fmt.Fprintf(procLog, "Process %s exited with error: %v\n", proc.Config.Name, err)

			if proc.Config.Restart == config.Never {
				updateStatus(s, process.StatusCrashed, proc)
				return
			}

			updateStatus(s, process.StatusRestarting, proc)

			time.Sleep(backoff)
			backoff *= 2
			if backoff >= 300*time.Second {
				backoff = 30 * time.Second
			}

			continue
		}

		if proc.Config.Restart == config.Never || proc.Config.Restart == config.UnlessStopped {
			updateStatus(s, process.StatusStopped, proc)
			return
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

func (s *Supervisor) StopProcesses() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, proc := range s.processes {
		if err := proc.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// TODO: this should be piped to the supervisor's log file
			log.Printf("failed to send sigterm: %v\n", err)
		}
	}
}

func (s *Supervisor) runLogRotation() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, p := range s.processes {
				s.maybeRotate(p)
			}
		}
	}
}

func (s *Supervisor) maybeRotate(proc *process.Process) {
	info, err := proc.LogFile.Stat()
	if err != nil || info.Size() < int64(s.maxLogSize) {
		return
	}

	oldLogFile := fmt.Sprintf("%s.1.log", proc.Config.Name)
	procLogFile := fmt.Sprintf("%s.log", proc.Config.Name)
	err = os.Rename(procLogFile, oldLogFile)
	if err != nil {
		log.Printf("couldn't rename file: %v", err)
		return
	}

	f, err := os.OpenFile(procLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening log file:", err)
		return
	}

	oldFile := proc.LogFile
	proc.LogFile = f
	proc.Cmd.Stderr = f
	proc.Cmd.Stdout = f
	oldFile.Close()
}

func updateStatus[T process.ProcessStatus](supervisor *Supervisor, status T, proc *process.Process) {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()

	proc.Status = process.ProcessStatus(status)
	proc.UpdatedAt = time.Now()
}
