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

type keyedTestRunner struct {
	testRunner
	key string
}

func (r testRunner) Name() string {
	return r.name
}

func (r keyedTestRunner) RunnerKey() string {
	return r.key
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

func TestRegistry_RunDeduplicatesRunnerAliases(t *testing.T) {
	registry := NewRegistry()
	started := make(chan string, 2)
	if err := registry.Register(keyedTestRunner{
		testRunner: testRunner{name: "legacy", started: started},
		key:        "shared",
	}); err != nil {
		t.Fatalf("register legacy: %v", err)
	}
	if err := registry.Register(keyedTestRunner{
		testRunner: testRunner{name: "alias", started: started},
		key:        "shared",
	}); err != nil {
		t.Fatalf("register alias: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- registry.Run(ctx, []string{"legacy", "alias"}, testLogger())
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected one aliased runner to start")
	}

	select {
	case name := <-started:
		t.Fatalf("expected aliases to start once, got second runner %q", name)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("registry did not stop aliased runner")
	}
}

func TestRegistry_RunUnknownConsumer(t *testing.T) {
	registry := NewRegistry()
	err := registry.Run(context.Background(), []string{"missing"}, testLogger())
	if err == nil {
		t.Fatal("expected unknown consumer error")
	}
}

func TestAliasRunnerUsesWrappedRunnerIdentity(t *testing.T) {
	started := make(chan string, 1)
	inner := testRunner{name: "analytics_events", started: started}
	alias := newAliasRunner("analytics.clickhouse_sync", inner)

	if alias.Name() != "analytics.clickhouse_sync" {
		t.Fatalf("unexpected alias name: %q", alias.Name())
	}

	keyed, ok := alias.(keyedRunner)
	if !ok {
		t.Fatal("expected alias runner to implement keyedRunner")
	}
	if keyed.RunnerKey() != "analytics_events" {
		t.Fatalf("unexpected alias runner key: %q", keyed.RunnerKey())
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
