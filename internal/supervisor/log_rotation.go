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
	mu         sync.Mutex
	file       *os.File
	path       string
	maxLogSize int
}

func NewLogWriter() *LogWriter {
	return &LogWriter{
		maxLogSize: 10 * (1 << 20),
		path:       configureLogPath(),
	}
}

func (s *Supervisor) RunLogRotation() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			for _, p := range s.processes {
				logWriter, ok := p.LogFile.(*LogWriter)
				if !ok {
					continue
				}
				logWriter.maybeRotate(p)
			}
			s.mu.Unlock()
		}
	}
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

func (l *LogWriter) maybeRotate(proc *process.Process) {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentFile := l.file

	info, err := currentFile.Stat()
	if err != nil {
		log.Printf("failed to stat log file for process %q: %v", proc.Config.Name, err)
		return
	}

	if info.Size() < int64(l.maxLogSize) {
		return
	}

	oldLogFileName := filepath.Join(l.path, proc.Config.Name+".1.log")
	procLogFileName := filepath.Join(l.path, proc.Config.Name+".log")

	err = os.Rename(procLogFileName, oldLogFileName)
	if err != nil {
		log.Printf("failed to rotate log file %q to %q for process %q: %v", procLogFileName, oldLogFileName, proc.Config.Name, err)
		return
	}

	f, err := os.OpenFile(procLogFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("failed to open new log file %q for process %q: %v", procLogFileName, proc.Config.Name, err)
		return
	}

	if err := currentFile.Close(); err != nil {
		log.Printf("failed to close rotated log file %q for process %q: %v", oldLogFileName, proc.Config.Name, err)
		f.Close()
		return
	}

	l.file = f
}
