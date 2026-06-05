package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type testRunner struct {
	name    string
	started chan string
}

func (r testRunner) Name() string {
	return r.name
}

func (r testRunner) Run(ctx context.Context) error {
	r.started <- r.name
	<-ctx.Done()
	return nil
}

func TestRegistry_RunWithZeroEnabledConsumers(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- registry.Run(ctx, nil, testLogger())
	}()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("registry did not stop after context cancellation")
	}
}

func TestRegistry_RunSelectedConsumers(t *testing.T) {
	registry := NewRegistry()
	started := make(chan string, 2)
	if err := registry.Register(testRunner{name: "delivery", started: started}); err != nil {
		t.Fatalf("register delivery: %v", err)
	}
	if err := registry.Register(testRunner{name: "analytics", started: started}); err != nil {
		t.Fatalf("register analytics: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- registry.Run(ctx, []string{"delivery", "analytics"}, testLogger())
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-time.After(time.Second):
			t.Fatal("registered consumers did not start")
		}
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("registry did not stop registered consumers")
	}
}

func TestRegistry_RunUnknownConsumer(t *testing.T) {
	registry := NewRegistry()
	err := registry.Run(context.Background(), []string{"missing"}, testLogger())
	if err == nil {
		t.Fatal("expected unknown consumer error")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
