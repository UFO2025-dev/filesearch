package logger

import (
	"log/slog"
	"os"
)

// Init sets up the global slog logger.
func Init(jsonFormat bool) {
	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	} else {
		handler = slog.NewTextHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(handler))
}
