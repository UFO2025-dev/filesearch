package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
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
// appends to %APPDATA%\FileSearch\filesearch.log.
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
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return os.Stderr
	}
	// Tee: write to both stderr (visible in dev) and file (visible to users)
	return io.MultiWriter(os.Stderr, f)
}

// windowsLogPath returns %APPDATA%\FileSearch\filesearch.log.
func windowsLogPath() string {
	dir, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "FileSearch", "filesearch.log")
}
