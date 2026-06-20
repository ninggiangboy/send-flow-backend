package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

// ElectedSchedulerRunner wraps a PollingWorker with a PostgreSQL advisory lock.
// Only one worker instance executes the scheduler poll cycle at a time.
type ElectedSchedulerRunner struct {
	name  string
	inner PollingWorker
	lock  *postgres.AdvisoryLock
	guard *PollingGuard
	log   *slog.Logger
}

func NewElectedSchedulerRunner(
	name string,
	inner PollingWorker,
	lock *postgres.AdvisoryLock,
	base time.Duration,
	log *slog.Logger,
) *ElectedSchedulerRunner {
	return &ElectedSchedulerRunner{
		name:  name,
		inner: inner,
		lock:  lock,
		guard: NewPollingGuard(name, base, 0, base, log),
		log:   log.With(logFieldElectedScheduler, name),
	}
}

func (e *ElectedSchedulerRunner) Name() string { return e.name }

func (e *ElectedSchedulerRunner) Run(ctx context.Context) error {
	return e.guard.Run(ctx, e)
}

func (e *ElectedSchedulerRunner) Poll(ctx context.Context) (bool, error) {
	ok, err := e.lock.TryLock(ctx)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	defer func() {
		if unlockErr := e.lock.Unlock(ctx); unlockErr != nil {
			e.log.Warn("failed to release advisory lock", "error", unlockErr)
		}
	}()
	return e.inner.Poll(ctx)
}
