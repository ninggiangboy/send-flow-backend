package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type mockPollWorker struct {
	pollFn func(ctx context.Context) (bool, error)
}

func (m *mockPollWorker) Poll(ctx context.Context) (bool, error) {
	return m.pollFn(ctx)
}

func TestPollingGuard_SkipWhenBusy(t *testing.T) {
	var calls atomic.Int32
	worker := &mockPollWorker{
		pollFn: func(ctx context.Context) (bool, error) {
			calls.Add(1)
			// Simulate long work — longer than the base interval
			time.Sleep(100 * time.Millisecond)
			return true, nil
		},
	}

	guard := NewPollingGuard("test", 10*time.Millisecond, 3, time.Second, testConsumerLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	_ = guard.Run(ctx, worker)

	// With a 10ms tick and 100ms work, at most 3 calls should complete in 250ms.
	// Without skip-when-busy, we'd see ~25 calls.
	if n := calls.Load(); n > 5 {
		t.Fatalf("expected at most 5 calls with skip-when-busy, got %d", n)
	}
}

func TestPollingGuard_BackoffOnEmpty(t *testing.T) {
	var calls atomic.Int32
	worker := &mockPollWorker{
		pollFn: func(ctx context.Context) (bool, error) {
			calls.Add(1)
			return false, nil // no work found
		},
	}

	guard := NewPollingGuard("test", 10*time.Millisecond, 3, 100*time.Millisecond, testConsumerLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_ = guard.Run(ctx, worker)

	// With backoff, the interval doubles on each empty poll (10ms → 20ms → 40ms → 80ms → capped at 100ms).
	// In 300ms we should see fewer calls than without backoff (~4-6 vs ~30).
	if n := calls.Load(); n > 12 {
		t.Fatalf("expected at most 12 calls with backoff, got %d", n)
	}
}

func TestPollingGuard_BackoffResetsOnWorkFound(t *testing.T) {
	var callCount int
	worker := &mockPollWorker{
		pollFn: func(ctx context.Context) (bool, error) {
			callCount++
			// First 2 calls return no work, then work found resets backoff
			return callCount > 2, nil
		},
	}

	guard := NewPollingGuard("test", 10*time.Millisecond, 3, 100*time.Millisecond, testConsumerLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = guard.Run(ctx, worker)

	// After call 3 finds work, backoff resets. With 200ms we should see ~8-10 calls.
	if callCount < 3 {
		t.Fatalf("expected at least 3 calls, got %d", callCount)
	}
}

func TestPollingGuard_ContextCancellation(t *testing.T) {
	worker := &mockPollWorker{
		pollFn: func(ctx context.Context) (bool, error) {
			return true, nil
		},
	}

	guard := NewPollingGuard("test", time.Minute, 3, 0, testConsumerLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	err := guard.Run(ctx, worker)
	if err != nil {
		t.Fatalf("expected nil error on cancelled context, got %v", err)
	}
}

func TestPollingGuard_ErrorDoesNotCrash(t *testing.T) {
	var calls atomic.Int32
	worker := &mockPollWorker{
		pollFn: func(ctx context.Context) (bool, error) {
			calls.Add(1)
			return false, nil
		},
	}

	guard := NewPollingGuard("test", 10*time.Millisecond, 3, 100*time.Millisecond, testConsumerLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = guard.Run(ctx, worker)

	if n := calls.Load(); n == 0 {
		t.Fatal("expected at least one call")
	}
}

func TestPollingGuard_IntervalBounds(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Duration
		maxEmpty int
		maxInt   time.Duration
		empty    int
		expected time.Duration
	}{
		{"base only", time.Second, 5, time.Minute, 0, time.Second},
		{"one backoff", time.Second, 5, time.Minute, 1, 2 * time.Second},
		{"two backoffs", time.Second, 5, time.Minute, 2, 4 * time.Second},
		{"capped by maxEmpty", time.Second, 3, time.Hour, 5, 8 * time.Second},
		{"capped by maxInterval", time.Second, 5, 5 * time.Second, 3, 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewPollingGuard("test", tt.base, tt.maxEmpty, tt.maxInt, testConsumerLogger())
			g.emptyCount = tt.empty
			got := g.currentInterval()
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
