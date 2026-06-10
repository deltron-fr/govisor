package supervisor

import (
	"fmt"
	"log"
	"os"
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

	oldLogFile := fmt.Sprintf("%s.1.log", proc.Config.Name)
	procLogFile := fmt.Sprintf("%s.log", proc.Config.Name)
	err = os.Rename(procLogFile, oldLogFile)
	if err != nil {
		log.Printf("couldn't rename file: %v", err)
		return
	}

	f, err := os.OpenFile(procLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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
