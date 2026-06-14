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
	attrs := []any{"app", cfg.AppName, "env", cfg.AppEnv}
	if cfg.RuntimeIdentity.ServiceName != "" {
		attrs = append(attrs, "service", cfg.RuntimeIdentity.ServiceName)
	}
	if cfg.RuntimeIdentity.InstanceID != "" {
		attrs = append(attrs, "instance_id", cfg.RuntimeIdentity.InstanceID)
	}
	if cfg.RuntimeIdentity.InstanceAddr != "" {
		attrs = append(attrs, "instance_addr", cfg.RuntimeIdentity.InstanceAddr)
	}
	if cfg.RuntimeIdentity.PodName != "" {
		attrs = append(attrs, "pod_name", cfg.RuntimeIdentity.PodName)
	}
	if cfg.RuntimeIdentity.Namespace != "" {
		attrs = append(attrs, "namespace", cfg.RuntimeIdentity.Namespace)
	}
	if cfg.RuntimeIdentity.NodeName != "" {
		attrs = append(attrs, "node_name", cfg.RuntimeIdentity.NodeName)
	}
	return slog.New(handler).With(attrs...)
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
