package supervisor

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/deltron-fr/govisor/internal/process"
)

type LogWriter struct {
	mu   sync.Mutex
	file *os.File
}

func (l *LogWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.file.Write(p)
}

func (l *LogWriter) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.file.Close()
}

func (s *Supervisor) RunLogRotation() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			for _, p := range s.processes {
				s.maybeRotate(p)
			}
			s.mu.Unlock()
		}
	}
}

func (s *Supervisor) maybeRotate(proc *process.Process) {
	logWriter, ok := proc.LogFile.(*LogWriter)
	if !ok {
		log.Printf("couldn't rotate log for process %s: invalid log writer", proc.Config.Name)
		return
	}

	currentFile := logWriter.file

	info, err := currentFile.Stat()
	if err != nil {
		log.Printf("couldn't get the necessary file info: %v", err)
		return
	}

	if info.Size() < int64(s.maxLogSize) {
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

	if err := currentFile.Close(); err != nil {
		log.Printf("couldn't close rotated log file: %v", err)
		f.Close()
		return
	}

	logWriter.file = f
}
