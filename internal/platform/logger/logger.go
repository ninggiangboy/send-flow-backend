package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

func New(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: parseLevel(cfg.LogLevel),
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler).With("app", cfg.AppName, "env", cfg.AppEnv)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
