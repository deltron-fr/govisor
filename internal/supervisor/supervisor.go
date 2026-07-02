package supervisor

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/deltron-fr/govisor/internal/config"
	"github.com/deltron-fr/govisor/internal/process"
)

type Supervisor struct {
	processConfigSet map[string][]*process.Process // maps config-name to the set of processes under it
	processes        map[string]*process.Process   // maps a single process name to its process object
	maxLogSize       int
	configFilePath   string
	logFilePath      string
	wg               sync.WaitGroup
	mu               sync.RWMutex
}

func NewSupervisor() *Supervisor {
	return &Supervisor{
		processConfigSet: make(map[string][]*process.Process),
		processes:        make(map[string]*process.Process),
		maxLogSize:       10 * (1 << 20),
		logFilePath:      configureLogPath(),
	}
}

// Apply takes the process configuration from the provided ConfigFile(parsed)
// and starts managing the processes accordingly.
// It initializes the process runtimes, starts them, and sets up log rotation.
// The method returns an error if there is an issue during the application of the configuration.
func (s *Supervisor) Apply(pfInfo config.ConfigFile) error {
	processes := make([]*process.Process, 0, len(pfInfo.Processes))

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.processConfigSet[pfInfo.Name]; ok {
		return fmt.Errorf("process configuration with the given name already exists")
	}

	for _, procConfig := range pfInfo.Processes {
		p := &process.Process{
			Config: procConfig,
			Status: process.StatusStarting,
		}
		s.processes[procConfig.Name] = p
		processes = append(processes, p)
	}
	s.processConfigSet[pfInfo.Name] = processes

	go s.runProcesses(processes)

	go s.runLogRotation()

	return nil
}

// runProcesses starts the given processes and manages their lifecycle.
func (s *Supervisor) runProcesses(processes []*process.Process) {
	for _, proc := range processes {
		s.wg.Add(1)
		proc.CreatedAt = time.Now()
		proc.UpdatedAt = time.Now()

		procLogFileName := filepath.Join(s.logFilePath, proc.Config.Name+".log")
		f, err := os.OpenFile(procLogFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("Failed to open log file for process %s: %v\n", proc.Config.Name, err)
			f = os.Stderr
		}

		procLog := f
		proc.LogFile = procLog
		proc.LogFileName = procLogFileName

		go func(proc *process.Process, procLog io.Writer) {
			s.runProcess(proc, procLog)
		}(proc, procLog)
	}

	s.wg.Wait()
}

// runProcess manages the full lifecycle of a supervised process, including startup,
// monitoring, and restart logic. It runs in its own goroutine and blocks until the
// process exits without requesting a restart.
//
// On exit or error, restart behavior is governed by the process Restart policy:
//   - Never: the process is marked crashed or stopped and the goroutine returns.
//   - UnlessStopped: restarts automatically unless the process exited cleanly.
//   - Always: restarts on both clean and unclean exits.
//
// Failed starts and command preparation errors are both subject to the same restart
// policy. Restart attempts use exponential backoff starting at 1s, capped at 30s
// once the 5-minute ceiling is reached.
//
// procLog receives all process stdout, stderr, and supervisor-level diagnostic messages
// for this process. If procLog is not os.Stderr, it is closed when the goroutine exits.
func (s *Supervisor) runProcess(proc *process.Process, procLog io.Writer) {
	defer func() {
		s.wg.Done()
		if procLog != os.Stderr {
			procLog.(*os.File).Close()
		}
	}()

	backoff := time.Second

	for {
		cmd, err := s.buildCommand(proc)
		if err != nil {
			fmt.Fprintf(procLog, "Failed to prepare process %s: %v\n", proc.Config.Name, err)

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

		cmd.Stdout = procLog
		cmd.Stderr = procLog
		if vars := parseEnvVars(proc.Config.Env); vars != nil {
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env, vars...)
		}

		err = cmd.Start()
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

		s.setProcessCommand(proc, cmd)
		backoff = time.Second

		updateStatus(s, process.StatusRunning, proc)

		err = cmd.Wait()
		s.clearProcessCommand(proc)
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

func (s *Supervisor) RetrieveLogs(proc *process.Process) ([]byte, error) {
	var bytesToRead int64 = 4096

	file, err := os.Open(proc.LogFileName)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if info.Size() < bytesToRead {
		bytesToRead = info.Size()
	}

	_, err = file.Seek(-bytesToRead, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, bytesToRead)
	_, err = io.ReadFull(file, buf)
	if err != nil {
		return nil, err
	}

	return buf, nil
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
		if proc.Cmd == nil || proc.Cmd.Process == nil {
			continue
		}

		if err := proc.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// TODO: this should be piped to the supervisor's log file
			log.Printf("failed to send sigterm: %v\n", err)
		}
	}
}

// buildCommand constructs an exec.Cmd for the given process based on its config.
// If the process is configured to run in a shell, it wraps the command with "sh -c"
// and uses exec to replace the shell process, ensuring signals are delivered directly
// to the command rather than the shell. Otherwise, it resolves the command path
// and builds the command with its arguments(if any).
func (s *Supervisor) buildCommand(proc *process.Process) (*exec.Cmd, error) {
	workDir := s.resolveWorkDir(proc.Config)

	if proc.Config.Shell {
		cmd := exec.Command("sh", "-c", "exec "+proc.Config.Command)
		cmd.Dir = workDir
		return cmd, nil
	}

	resolvedCommand, err := s.resolveCommandPath(proc.Config.Command, workDir)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(resolvedCommand, proc.Config.Args...)
	cmd.Dir = workDir
	return cmd, nil
}

// resolveWorkDir determines the working directory for a process.
// It follows this precedence:
//  1. An explicit WorkDir in the process config, resolved relative to the config base dir.
//  2. The config base dir for shell commands (since the command string may reference relative paths).
//  3. An empty string for absolute command paths or commands found on PATH, letting the OS use
//     the supervisor's own working directory.
//  4. The config base dir as a fallback for relative command paths not found on PATH.
func (s *Supervisor) resolveWorkDir(procConfig config.ProcessConfig) string {
	baseDir := s.configBaseDir()

	if procConfig.WorkDir != "" {
		return resolvePath(procConfig.WorkDir, baseDir)
	}

	if procConfig.Shell {
		return baseDir
	}

	if filepath.IsAbs(procConfig.Command) {
		return ""
	}

	if !hasPathSeparator(procConfig.Command) && commandExistsInPath(procConfig.Command) {
		return ""
	}

	return baseDir
}

// resolveCommandPath returns the absolute path to the executable for the given command.
//  1. Absolute paths are cleaned and returned as-is.
//  2. Relative paths containing a separator are resolved against workDir (or the config
//     base dir if workDir is empty).
//  3. Plain command names (no separator) are looked up via PATH.
//  4. If PATH lookup fails, the command is resolved relative to workDir.
func (s *Supervisor) resolveCommandPath(command string, workDir string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	if filepath.IsAbs(command) {
		return filepath.Clean(command), nil
	}

	if hasPathSeparator(command) {
		return resolvePath(command, firstNonEmpty(workDir, s.configBaseDir())), nil
	}

	if resolvedCommand, err := exec.LookPath(command); err == nil {
		return resolvedCommand, nil
	}

	return resolvePath(command, firstNonEmpty(workDir, s.configBaseDir())), nil
}

func parseEnvVars(env map[string]string) []string {
	if env == nil {
		return nil
	}

	var envVars []string
	for key, value := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	return envVars
}

func updateStatus[T process.ProcessStatus](supervisor *Supervisor, status T, proc *process.Process) {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()

	proc.Status = process.ProcessStatus(status)
	proc.UpdatedAt = time.Now()
}

func (s *Supervisor) setProcessCommand(proc *process.Process, cmd *exec.Cmd) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proc.Cmd = cmd
}

func (s *Supervisor) clearProcessCommand(proc *process.Process) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proc.Cmd = nil
}
