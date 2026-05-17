package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	maxLogSize    = 10 * 1024 * 1024 // 10 MB per file
	maxLogBackups = 3                 // keep .1 .2 .3
)

// Init sets up the global slog logger.
// On Windows (windowsgui build) stdout/stderr are discarded, so we always
// tee logs to %APPDATA%\FileSearch\filesearch.log for crash reporting.
func Init(jsonFormat bool) {
	out := logWriter()
	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(out, nil)
	} else {
		handler = slog.NewTextHandler(out, nil)
	}
	slog.SetDefault(slog.New(handler))
}

// logWriter returns a writer that writes to stderr and, on Windows, also
// appends to %APPDATA%\FileSearch\filesearch.log with size-based rotation.
func logWriter() io.Writer {
	if runtime.GOOS != "windows" {
		return os.Stderr
	}
	logPath := windowsLogPath()
	if logPath == "" {
		return os.Stderr
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return os.Stderr
	}
	rw, err := newRotatingWriter(logPath)
	if err != nil {
		return os.Stderr
	}
	// Tee: write to both stderr (visible in dev) and rotating file (visible to users).
	return io.MultiWriter(os.Stderr, rw)
}

// windowsLogPath returns %APPDATA%\FileSearch\filesearch.log.
func windowsLogPath() string {
	dir, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "FileSearch", "filesearch.log")
}

// rotatingWriter writes to a log file and rotates it when it exceeds maxLogSize.
type rotatingWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

func newRotatingWriter(path string) (*rotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	var sz int64
	if info, err := f.Stat(); err == nil {
		sz = info.Size()
	}
	return &rotatingWriter{path: path, file: f, size: sz}, nil
}

func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > maxLogSize {
		w.rotate()
	}
	n, err = w.file.Write(p)
	w.size += int64(n)
	return
}

// rotate shifts existing backup files and opens a fresh log file.
func (w *rotatingWriter) rotate() {
	_ = w.file.Close()
	// Shift: .3 is dropped, .2→.3, .1→.2, current→.1
	for i := maxLogBackups - 1; i >= 1; i-- {
		_ = os.Rename(
			fmt.Sprintf("%s.%d", w.path, i),
			fmt.Sprintf("%s.%d", w.path, i+1),
		)
	}
	_ = os.Rename(w.path, w.path+".1")
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		// Fallback: reopen in append mode
		f, _ = os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	}
	w.file = f
	w.size = 0
}
