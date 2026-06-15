package postgres

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLockBusy     = errors.New("advisory lock already held by this instance")
	ErrLockNotHeld  = errors.New("advisory lock not held")
	ErrLockConflict = errors.New("advisory lock held by another instance")
)

// AdvisoryLock provides cross-instance coordination using PostgreSQL advisory locks.
// TryLock acquires a dedicated connection from the pool and holds it for the lock
// duration. The lock is automatically released if the connection drops or the
// instance crashes. Only one instance can hold a given lock at a time.
type AdvisoryLock struct {
	pool   *pgxpool.Pool
	conn   *pgxpool.Conn
	lockID int64
	mu     sync.Mutex
}

// NewAdvisoryLock creates an AdvisoryLock for the given name.
// The lock ID is derived from the name using FNV-1a 64-bit hash.
func NewAdvisoryLock(pool *pgxpool.Pool, name string) *AdvisoryLock {
	return &AdvisoryLock{
		pool:   pool,
		lockID: hashToInt64(name),
	}
}

// TryLock attempts to acquire the advisory lock non-blocking.
// Returns:
//
//	true, nil  - lock acquired, caller is the leader
//	false, nil - lock held by another instance
//	false, err - error acquiring the lock
//
// After TryLock returns true, the caller MUST call Unlock when done.
func (l *AdvisoryLock) TryLock(ctx context.Context) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.conn != nil {
		return false, ErrLockBusy
	}

	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", l.lockID).Scan(&acquired); err != nil {
		conn.Release()
		return false, err
	}

	if !acquired {
		conn.Release()
		return false, nil
	}

	l.conn = conn
	return true, nil
}

// Unlock releases the advisory lock and returns the connection to the pool.
func (l *AdvisoryLock) Unlock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.conn == nil {
		return ErrLockNotHeld
	}

	unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if _, err := l.conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", l.lockID); err != nil {
		l.conn.Release()
		l.conn = nil
		return err
	}

	l.conn.Release()
	l.conn = nil
	return nil
}

func hashToInt64(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64())
}
