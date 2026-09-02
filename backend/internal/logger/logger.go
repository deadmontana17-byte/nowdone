// Package logger configures the shared slog logger with Debug/Info/Warn/Error levels.
package logger

import (
	"log/slog"
	"os"
)

// New creates a JSON slog logger writing to stdout. Level is derived from env.
func New(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
