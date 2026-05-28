package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/deltron-fr/govisor/internal/config"
	"github.com/deltron-fr/govisor/internal/ipc"
	"github.com/deltron-fr/govisor/internal/process"
	"github.com/deltron-fr/govisor/internal/supervisor"
	"go.yaml.in/yaml/v4"
)

type Server struct {
	socketPath string
	supervisor *supervisor.Supervisor
}

func New(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		supervisor: supervisor.NewSupervisor(),
	}
}

func (s *Server) Serve() {
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o755); err != nil {
		log.Fatalf("failed to create socket directory: %v", err)
	}

	_ = os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on socket: %v", err)
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /apply", s.handleApply)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /stop", s.handleStop)

	srv := http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      mux,
	}

	go func() {
		if err := srv.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	<-sigChan
	fmt.Println("received signal. shutting down gracefully...")
	s.supervisor.StopProcesses()

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		log.Printf("Failed to shutdown server gracefully: %v", err)
	}

	os.Remove(ipc.DEFAULT_SOCKET_PATH)
	log.Println("socket has been removed. exiting.")
}

func (s *Server) handleApply(w http.ResponseWriter, req *http.Request) {
	filepath, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	s.supervisor.SetConfigFilePath(string(filepath))

	bodyBytes, err := os.ReadFile(string(filepath))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Config file not found", http.StatusUnprocessableEntity)
			return
		}

		if os.IsPermission(err) {
			http.Error(w, "Permission denied reading config file", http.StatusForbidden)
			return
		}

		http.Error(w, "Failed to read config file", http.StatusInternalServerError)
		return
	}

	var pcInfo config.ConfigFile
	if err := yaml.Unmarshal(bodyBytes, &pcInfo); err != nil {
		http.Error(w, "Failed to parse YAML: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = s.supervisor.Apply(pcInfo)
	if err != nil {
		http.Error(w, "Failed to start supervisor: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Process configuration applied successfully\n"))
}

func (s *Server) handleStop(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("stopping all processes...\n"))

	s.supervisor.StopProcesses()

	w.Write([]byte("all processes stopped successfully\n"))
}

func (s *Server) handleStatus(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	s.writeStatus(w, s.supervisor.Snapshots())
}

func (s *Server) writeStatus(writer io.Writer, snapshots []process.Snapshot) {
	tw := tabwriter.NewWriter(writer, 0, 0, 3, ' ', 0)

	fmt.Fprintln(tw, "NAME\tSTATUS\tCOMMAND\tCREATED\tUPDATED")

	for _, s := range snapshots {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			s.Name,
			s.Status.String(),
			s.Command,
			s.CreatedAt.Format("15:04:05"),
			s.UpdatedAt.Format("15:04:05"),
		)
	}

	tw.Flush()
}
