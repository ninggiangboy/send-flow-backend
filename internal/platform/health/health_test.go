package health

import (
	"context"
	"errors"
	"testing"
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
		ClickEnabled:         true,
		ObjectStorageEnabled: true,
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
