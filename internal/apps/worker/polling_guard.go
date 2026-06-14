package worker

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

const (
	defaultMaxEmptyCount = 5
	defaultMaxInterval   = 5 * time.Minute
)

// PollingWorker is implemented by poller processors that want overlap protection and idle backoff.
type PollingWorker interface {
	// Poll performs a single cycle of work. Returns workFound=true if there was
	// actual work to do (used to reset the idle backoff counter).
	Poll(ctx context.Context) (workFound bool, err error)
}

// PollingGuard wraps a PollingWorker with skip-when-busy and idle backoff logic.
type PollingGuard struct {
	name        string
	base        time.Duration
	maxInterval time.Duration
	maxEmpty    int
	running     atomic.Bool
	emptyCount  int
	log         *slog.Logger
}

// NewPollingGuard creates a guard.
// base — normal interval between polls.
// maxEmpty — number of consecutive empty polls before reaching maxInterval (via exponential backoff).
// If maxEmpty <= 0 it defaults to defaultMaxEmptyCount. If maxInterval <= 0 it defaults to defaultMaxInterval.
func NewPollingGuard(name string, base time.Duration, maxEmpty int, maxInterval time.Duration, log *slog.Logger) *PollingGuard {
	if maxEmpty <= 0 {
		maxEmpty = defaultMaxEmptyCount
	}
	if maxInterval <= 0 {
		maxInterval = defaultMaxInterval
	}
	if base <= 0 {
		base = time.Second
	}
	return &PollingGuard{
		name:        name,
		base:        base,
		maxInterval: maxInterval,
		maxEmpty:    maxEmpty,
		log:         log.With("polling_guard", name),
	}
}

// Run blocks until ctx is cancelled, calling worker.Poll on each tick.
func (g *PollingGuard) Run(ctx context.Context, worker PollingWorker) error {
	timer := time.NewTimer(g.base)
	defer timer.Stop()

	g.log.Info("starting poller",
		"base_interval", g.base,
		"max_interval", g.maxInterval,
	)

	for {
		select {
		case <-ctx.Done():
			g.log.Info("poller stopped")
			return nil

		case <-timer.C:
			if !g.running.CompareAndSwap(false, true) {
				g.log.Debug("previous poll still running, skipping tick")
				timer.Reset(g.currentInterval())
				continue
			}

			func() {
				defer g.running.Store(false)
				workFound, err := worker.Poll(ctx)
				if err != nil {
					g.log.Error("poll cycle failed", "error", err)
				}
				if workFound {
					g.emptyCount = 0
				} else {
					g.emptyCount++
				}
				timer.Reset(g.currentInterval())
			}()
		}
	}
}

// currentInterval returns the effective interval based on consecutive empty polls.
// Backoff: base * 2^min(emptyCount, maxEmpty), capped at maxInterval.
func (g *PollingGuard) currentInterval() time.Duration {
	n := g.emptyCount
	if n > g.maxEmpty {
		n = g.maxEmpty
	}
	d := g.base * time.Duration(1<<n) // 2^N
	if d > g.maxInterval {
		d = g.maxInterval
	}
	return d
}
