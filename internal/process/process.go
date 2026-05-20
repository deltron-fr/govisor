package process

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"text/tabwriter"
	"time"

	"go.yaml.in/yaml/v4"
)

var SOCKET_FILE_PATH = "/tmp/govisor.sock"

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
	Cmd       *exec.Cmd
	Config    ProcessConfig
	Status    ProcessStatus
	CreatedAt time.Time
	UpdatedAt time.Time
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

func (s *Supervisor) StartSupervisor(pfInfo ProcessConfigFileInfo) error {
	for _, procConfig := range pfInfo.Processes {
		s.processes[procConfig.Name] = &Process{
			Config: procConfig,
			Status: StatusStarting,
		}
	}

	go s.startProcesses()

	return nil
}

func (s *Supervisor) startProcesses() {
	for _, proc := range s.processes {
		s.wg.Add(1)
		proc.Status = StatusStarting
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

		go func(proc *Process, procLog *os.File) {
			s.startProcess(proc, procLog)
		}(proc, procLog)
	}

	s.wg.Wait()
}

func (s *Supervisor) startProcess(proc *Process, procLog *os.File) {
	defer s.wg.Done()
	defer procLog.Close()

	backoff := time.Second

	for {
		cmd := exec.Command("sh", "-c", proc.Config.Command)
		cmd.Stdout = procLog
		cmd.Stderr = procLog

		err := cmd.Start()
		if err != nil {
			s.mu.Lock()
			proc.Status = StatusRestarting
			proc.UpdatedAt = time.Now()
			s.mu.Unlock()

			time.Sleep(backoff)
			backoff *= 2

			fmt.Fprintf(procLog, "Failed to start process %s: %v\n", proc.Config.Name, err)
			continue
		}

		backoff = time.Second

		s.mu.Lock()
		proc.Status = StatusRunning
		proc.UpdatedAt = time.Now()
		s.mu.Unlock()

		err = cmd.Wait()

		s.mu.Lock()
		if err != nil {
			fmt.Fprintf(procLog, "Process %s exited with error: %v\n", proc.Config.Name, err)

			proc.Status = StatusRestarting
			proc.UpdatedAt = time.Now()
			s.mu.Unlock()

			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		proc.Status = StatusStopped
		proc.UpdatedAt = time.Now()
		s.mu.Unlock()

		break
	}
}

func (s *Supervisor) monitorProcesses(writer io.Writer) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tw := tabwriter.NewWriter(writer, 0, 0, 3, ' ', 0)

	fmt.Fprintln(tw, "NAME\tSTATUS\tCOMMAND\tCREATED\tUPDATED")

	for _, proc := range s.processes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			proc.Config.Name,
			proc.Status.String(),
			proc.Config.Command,
			proc.CreatedAt.Format("15:04:05"),
			proc.UpdatedAt.Format("15:04:05"),
		)
	}

	tw.Flush()
}

func (s *Supervisor) ServeAPI() {
	_ = os.Remove(SOCKET_FILE_PATH)

	listener, err := net.Listen("unix", SOCKET_FILE_PATH)
	if err != nil {
		log.Fatalf("Failed to listen on socket: %v", err)
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /apply", s.handleApply)
	mux.HandleFunc("GET /status", s.handleStatus)

	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func (s *Supervisor) handleApply(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	var pcInfo ProcessConfigFileInfo
	if err := yaml.Unmarshal(bodyBytes, &pcInfo); err != nil {
		http.Error(w, "Failed to parse YAML: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = s.StartSupervisor(pcInfo)
	if err != nil {
		http.Error(w, "Failed to start supervisor: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Process configuration applied successfully"))
}

func (s *Supervisor) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	s.monitorProcesses(w)
}
