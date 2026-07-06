package supervisor

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deltron-fr/govisor/internal/config"
	"github.com/deltron-fr/govisor/internal/process"
)

func TestApplyRejectsDuplicateProcessNamesInSameConfig(t *testing.T) {
	supervisor := NewSupervisor()

	err := supervisor.Apply(config.ConfigFile{
		Name: "app",
		Processes: []config.ProcessConfig{
			{Name: "api", Command: "echo"},
			{Name: "api", Command: "printf"},
		},
	})

	if err == nil {
		t.Fatal("Apply() error = nil, want duplicate process name error")
	}

	if !strings.Contains(err.Error(), `duplicate process name "api"`) {
		t.Fatalf("Apply() error = %q, want duplicate process name error", err)
	}

	if len(supervisor.processes) != 0 {
		t.Fatalf("Apply() registered %d processes, want 0", len(supervisor.processes))
	}
}

func TestApplyRejectsProcessNameAlreadyManaged(t *testing.T) {
	supervisor := NewSupervisor()
	supervisor.processes["api"] = &process.Process{}

	err := supervisor.Apply(config.ConfigFile{
		Name: "app",
		Processes: []config.ProcessConfig{
			{Name: "api", Command: "echo"},
		},
	})

	if err == nil {
		t.Fatal("Apply() error = nil, want existing process name error")
	}

	if !strings.Contains(err.Error(), `process name "api" already exists`) {
		t.Fatalf("Apply() error = %q, want existing process name error", err)
	}

	if len(supervisor.processConfigSet) != 0 {
		t.Fatalf("Apply() registered %d process configs, want 0", len(supervisor.processConfigSet))
	}
}

func TestResolveWorkDirUsesConfiguredWorkDir(t *testing.T) {
	supervisor := &Supervisor{configFilePath: "/tmp/govisor/config.yaml"}

	got := supervisor.resolveWorkDir(config.ProcessConfig{WorkDir: "services/api"})
	if got != "/tmp/govisor/services/api" {
		t.Fatalf("resolveWorkDir() = %q, want %q", got, "/tmp/govisor/services/api")
	}
}

func TestResolveCommandPathRejectsEmptyCommand(t *testing.T) {
	supervisor := &Supervisor{}

	_, err := supervisor.resolveCommandPath("", "")
	if err == nil {
		t.Fatal("resolveCommandPath() error = nil, want error")
	}
}

func TestBuildCommandShellWrapsCommand(t *testing.T) {
	supervisor := &Supervisor{configFilePath: "/tmp/govisor/config.yaml"}
	proc := &process.Process{
		Config: config.ProcessConfig{
			Command: "bin/api --port 8080",
			Shell:   true,
		},
	}

	cmd, err := supervisor.buildCommand(proc)
	if err != nil {
		t.Fatalf("buildCommand() error = %v", err)
	}

	if got := filepath.Base(cmd.Path); got != "sh" {
		t.Fatalf("buildCommand() path = %q, want basename %q", cmd.Path, "sh")
	}

	if len(cmd.Args) != 3 || cmd.Args[1] != "-c" || cmd.Args[2] != "exec bin/api --port 8080" {
		t.Fatalf("buildCommand() args = %#v", cmd.Args)
	}

	if got := cmd.Dir; got != "/tmp/govisor" {
		t.Fatalf("buildCommand() dir = %q, want %q", got, "/tmp/govisor")
	}
}

func TestSetProcessCommand(t *testing.T) {
	supervisor := &Supervisor{}
	proc := &process.Process{}
	cmd := exec.Command("sleep", "1")

	supervisor.setProcessCommand(proc, cmd)

	if proc.Cmd != cmd {
		t.Fatal("setProcessCommand() did not set process command")
	}
}

func TestClearProcessCommand(t *testing.T) {
	supervisor := &Supervisor{}
	proc := &process.Process{Cmd: exec.Command("sleep", "1")}

	supervisor.clearProcessCommand(proc)

	if proc.Cmd != nil {
		t.Fatal("clearProcessCommand() did not clear process command")
	}
}

func TestUpdateStatus(t *testing.T) {
	supervisor := &Supervisor{}
	proc := &process.Process{
		Status:    process.StatusStarting,
		UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	before := proc.UpdatedAt

	updateStatus(supervisor, process.StatusRunning, proc)

	if proc.Status != process.StatusRunning {
		t.Fatalf("updateStatus() status = %v, want %v", proc.Status, process.StatusRunning)
	}

	if !proc.UpdatedAt.After(before) {
		t.Fatalf("updateStatus() UpdatedAt = %v, want after %v", proc.UpdatedAt, before)
	}
}

func TestRetrieveLogsReturnsLastFourKiB(t *testing.T) {
	const tailSize = 4096

	prefix := bytes.Repeat([]byte("x"), 128)
	want := bytes.Repeat([]byte("log\n"), tailSize/4)
	contents := append(prefix, want...)
	logFileName := filepath.Join(t.TempDir(), "api.log")

	if err := os.WriteFile(logFileName, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	supervisor := &Supervisor{}
	proc := &process.Process{LogFileName: logFileName}

	got, err := supervisor.RetrieveLogs(proc)
	if err != nil {
		t.Fatalf("RetrieveLogs() error = %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("RetrieveLogs() returned %d bytes with unexpected content, want final %d bytes", len(got), len(want))
	}
}

func TestRetrieveLogsReturnsEntireSmallLog(t *testing.T) {
	want := []byte("api started\nrequest handled\n")
	logFileName := filepath.Join(t.TempDir(), "api.log")

	if err := os.WriteFile(logFileName, want, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	supervisor := &Supervisor{}
	proc := &process.Process{LogFileName: logFileName}

	got, err := supervisor.RetrieveLogs(proc)
	if err != nil {
		t.Fatalf("RetrieveLogs() error = %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("RetrieveLogs() = %q, want %q", got, want)
	}
}

func TestRunProcessPassesEnvironmentToDirectCommand(t *testing.T) {
	t.Setenv("GOVISOR_TEST_INHERITED", "from-parent")
	t.Setenv("GOVISOR_TEST_OVERRIDE", "from-parent")

	proc := &process.Process{
		Config: config.ProcessConfig{
			Name:    "direct-environment",
			Command: "printenv",
			Args: []string{
				"GOVISOR_TEST_CONFIGURED",
				"GOVISOR_TEST_OVERRIDE",
				"GOVISOR_TEST_INHERITED",
			},
			Env: map[string]string{
				"GOVISOR_TEST_CONFIGURED": "from-config",
				"GOVISOR_TEST_OVERRIDE":   "from-config",
			},
			Restart: config.Never,
		},
	}

	got := runProcessAndReadLog(t, proc)
	want := "from-config\nfrom-config\nfrom-parent\n"
	if got != want {
		t.Fatalf("runProcess() log = %q, want %q", got, want)
	}
}

func TestRunProcessExpandsEnvironmentInShellCommand(t *testing.T) {
	t.Setenv("GOVISOR_TEST_INHERITED", "from-parent")
	t.Setenv("GOVISOR_TEST_OVERRIDE", "from-parent")

	proc := &process.Process{
		Config: config.ProcessConfig{
			Name: "shell-environment",
			Command: `printf '%s|%s|%s\n' \
"$GOVISOR_TEST_CONFIGURED" \
"$GOVISOR_TEST_OVERRIDE" \
"$GOVISOR_TEST_INHERITED"`,
			Env: map[string]string{
				"GOVISOR_TEST_CONFIGURED": "from-config",
				"GOVISOR_TEST_OVERRIDE":   "from-config",
			},
			Restart: config.Never,
			Shell:   true,
		},
	}

	got := runProcessAndReadLog(t, proc)
	want := "from-config|from-config|from-parent\n"
	if got != want {
		t.Fatalf("runProcess() log = %q, want %q", got, want)
	}
}

func runProcessAndReadLog(t *testing.T, proc *process.Process) string {
	t.Helper()

	logFileName := filepath.Join(t.TempDir(), proc.Config.Name+".log")
	logFile, err := os.Create(logFileName)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	proc.LogFile = &LogWriter{file: logFile}
	proc.LogFileName = logFileName

	supervisor := NewSupervisor()
	supervisor.wg.Add(1)
	supervisor.runProcess(proc)

	contents, err := os.ReadFile(logFileName)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if proc.Status != process.StatusStopped {
		t.Fatalf("runProcess() status = %v, want %v", proc.Status, process.StatusStopped)
	}

	return string(contents)
}
