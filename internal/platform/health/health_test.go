package health

import (
	"context"
	"errors"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/buildinfo"
)

func TestServiceReadyOK(t *testing.T) {
	svc := NewService(Options{
		AppName: "sendflow",
		PostgresCheck: func(context.Context) error {
			return nil
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
		ObjectStorageCheck: func(context.Context) error {
			return nil
		},
		KafkaEnabled:         true,
		ObjectStorageEnabled: true,
		Build: buildinfo.Info{
			Version:   "1.2.3",
			GitSHA:    "abc123",
			BuildTime: "2026-06-13T00:00:00Z",
		},
	})

	ready := svc.Ready(context.Background())
	if ready.Status != "ok" {
		t.Fatalf("expected ok, got %s", ready.Status)
	}
	if ready.Dependencies["postgres"] != "up" {
		t.Fatalf("expected postgres up")
	}
	if ready.Dependencies["redis"] != "up" {
		t.Fatalf("expected redis up")
	}
	if ready.Dependencies["object_storage"] != "up" {
		t.Fatalf("expected object storage up")
	}
	if ready.Version != "1.2.3" || ready.GitSHA != "abc123" || ready.BuildTime == "" {
		t.Fatalf("expected build metadata in readiness response, got %+v", ready)
	}
	live := svc.Live()
	if live.Version != "1.2.3" || live.GitSHA != "abc123" || live.BuildTime == "" {
		t.Fatalf("expected build metadata in liveness response, got %+v", live)
	}
}

func TestServiceReadyDegraded(t *testing.T) {
	svc := NewService(Options{
		AppName: "sendflow",
		PostgresCheck: func(context.Context) error {
			return errors.New("db down")
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
	})

	ready := svc.Ready(context.Background())
	if ready.Status != "degraded" {
		t.Fatalf("expected degraded, got %s", ready.Status)
	}
	if ready.Dependencies["postgres"] != "down" {
		t.Fatalf("expected postgres down")
	}
	if ready.Dependencies["object_storage"] != "disabled" {
		t.Fatalf("expected object storage disabled")
	}
}
