package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/deltron-fr/govisor/internal/config"
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
	_ = os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
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

func (s *Server) handleApply(w http.ResponseWriter, req *http.Request) {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
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
