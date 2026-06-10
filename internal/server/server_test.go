package server

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/deltron-fr/govisor/internal/process"
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
