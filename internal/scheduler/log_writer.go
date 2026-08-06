package scheduler

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// LogWriter streams command output to a log file and buffers the tail content
// in memory so recent logs can be served without reading the whole file.
type LogWriter struct {
	mu      sync.Mutex
	file    *os.File
	buffer  []byte
	maxSize int
}

const defaultBufferSize = 256 * 1024

func NewLogWriter(path string) (*LogWriter, error) {
	if err := os.MkdirAll(filepathDir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &LogWriter{
		file:    file,
		maxSize: defaultBufferSize,
	}, nil
}

func (w *LogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.file.Write(p)
	if len(w.buffer) < w.maxSize {
		remain := w.maxSize - len(w.buffer)
		if len(p) > remain {
			p = p[len(p)-remain:]
		}
		w.buffer = append(w.buffer, p...)
	}
	return n, err
}

func (w *LogWriter) Logf(format string, args ...interface{}) {
	msg := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), "INFO", fmt.Sprintf(format, args...))
	_, _ = w.Write([]byte(msg))
}

func (w *LogWriter) Content() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.buffer)
}

func (w *LogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
