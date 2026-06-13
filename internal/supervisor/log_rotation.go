package supervisor

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/deltron-fr/govisor/internal/process"
)

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

	oldLogFileName := filepath.Join(s.logFilePath, proc.Config.Name+".1.log")
	procLogFileName := filepath.Join(s.logFilePath, proc.Config.Name+".log")

	err = os.Rename(procLogFileName, oldLogFileName)
	if err != nil {
		log.Printf("couldn't rename file: %v", err)
		return
	}

	f, err := os.OpenFile(procLogFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("couldn't open new log file: %v", err)
		return
	}

	oldFile := proc.LogFile
	defer oldFile.Close()

	s.mu.Lock()
	proc.LogFile = f
	if proc.Cmd != nil {
		proc.Cmd.Stderr = f
		proc.Cmd.Stdout = f
	}
	s.mu.Unlock()
}
