package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deltron-fr/govisor/internal/config"
	"github.com/deltron-fr/govisor/internal/process"
	"github.com/deltron-fr/govisor/internal/supervisor"
)

func TestWriteStatusRendersSnapshots(t *testing.T) {
	server := &Server{}
	var out bytes.Buffer

	server.writeStatus(&out, []process.Snapshot{
		{
			Name:      "api",
			Command:   "bin/api",
			Status:    process.StatusRunning,
			CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt: time.Date(2026, 1, 2, 6, 7, 8, 0, time.UTC),
		},
	})

	got := out.String()
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "STATUS") || !strings.Contains(got, "COMMAND") {
		t.Fatalf("writeStatus() header missing:\n%s", got)
	}

	if !strings.Contains(got, "api") || !strings.Contains(got, "RUNNING") || !strings.Contains(got, "bin/api") {
		t.Fatalf("writeStatus() row missing:\n%s", got)
	}

	if !strings.Contains(got, "03:04:05") || !strings.Contains(got, "06:07:08") {
		t.Fatalf("writeStatus() timestamps missing:\n%s", got)
	}
}

func TestHandleLogsReturnsProcessLogs(t *testing.T) {
	const want = "api started\nrequest handled\n"

	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	logDir := filepath.Join(stateHome, "govisor", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	processSupervisor := supervisor.NewSupervisor()

	err := processSupervisor.Apply(config.ConfigFile{
		Processes: []config.ProcessConfig{
			{
				Name:    "api",
				Command: "printf",
				Args:    []string{want},
				Restart: config.Never,
			},
		},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	t.Cleanup(processSupervisor.StopProcesses)

	logFileName := filepath.Join(logDir, "api.log")
	deadline := time.Now().Add(2 * time.Second)
	for {
		contents, readErr := os.ReadFile(logFileName)
		if readErr == nil && string(contents) == want {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("process log did not contain %q before timeout; contents = %q, error = %v", want, contents, readErr)
		}

		time.Sleep(10 * time.Millisecond)
	}

	server := &Server{supervisor: processSupervisor}
	req := httptest.NewRequest(http.MethodGet, "/logs/api", nil)
	req.SetPathValue("proc_name", "api")
	recorder := httptest.NewRecorder()

	server.handleLogs(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("handleLogs() status = %d, want %d; body = %q", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	if got := recorder.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("handleLogs() Content-Type = %q, want %q", got, "text/plain")
	}

	if got := recorder.Body.String(); got != want {
		t.Fatalf("handleLogs() body = %q, want %q", got, want)
	}
}

func TestHandleLogsRejectsUnknownProcess(t *testing.T) {
	server := &Server{supervisor: supervisor.NewSupervisor()}
	req := httptest.NewRequest(http.MethodGet, "/logs/missing", nil)
	req.SetPathValue("proc_name", "missing")
	recorder := httptest.NewRecorder()

	server.handleLogs(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("handleLogs() status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	if got, want := recorder.Body.String(), "process \"missing\" does not exist\n"; got != want {
		t.Fatalf("handleLogs() body = %q, want %q", got, want)
	}
}
