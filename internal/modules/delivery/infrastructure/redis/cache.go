package redis

import (
	"context"
	"time"

	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

// IdempotencyEntry holds the information needed for delivery idempotency checks.
type IdempotencyEntry struct {
	PayloadHash string `json:"payload_hash"`
	RequestID   string `json:"request_id"`
	MessageID   string `json:"message_id"`
	Status      string `json:"status"`
	AcceptedAt  string `json:"accepted_at"`
}

// Cache provides delivery idempotency, quota, and lock functionality.
type Cache struct {
	aside    *platformredis.CacheAside
	client   *platformredis.Client
	idemTTL  time.Duration
	quotaTTL time.Duration
}

// NewCache returns a new Cache that wraps the given cache-aside backend and client.
func NewCache(aside *platformredis.CacheAside, client *platformredis.Client, idemTTL, quotaTTL time.Duration) *Cache {
	return &Cache{
		aside:    aside,
		client:   client,
		idemTTL:  idemTTL,
		quotaTTL: quotaTTL,
	}
}

// GetIdempotency retrieves a cached idempotency entry for the given workspace and
// idempotency key. Returns platformredis.ErrCacheMiss if not found.
func (c *Cache) GetIdempotency(ctx context.Context, workspaceID, idempotencyKey string) (*IdempotencyEntry, error) {
	key := platformredis.KeyDeliveryIdempotency(workspaceID, idempotencyKey)
	var entry IdempotencyEntry
	if err := c.client.GetJSON(ctx, key, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// SetIdempotency stores an idempotency entry in the cache after a successful DB write.
func (c *Cache) SetIdempotency(ctx context.Context, workspaceID, idempotencyKey string, entry *IdempotencyEntry) error {
	key := platformredis.KeyDeliveryIdempotency(workspaceID, idempotencyKey)
	return c.client.SetJSON(ctx, key, entry, c.idemTTL)
}

// DeleteIdempotency removes a cached idempotency entry.
func (c *Cache) DeleteIdempotency(ctx context.Context, workspaceID, idempotencyKey string) error {
	key := platformredis.KeyDeliveryIdempotency(workspaceID, idempotencyKey)
	_, err := c.client.Delete(ctx, key)
	return err
}

// CheckQuota implements a simple sliding window rate limiter using a Redis counter.
// It increments the counter for the given provider and workspace, sets an expiry on
// the first increment within a window, and returns whether the request is allowed
// along with the remaining quota.
func (c *Cache) CheckQuota(ctx context.Context, workspaceID, provider string, max int64) (allowed bool, remaining int64, err error) {
	key := platformredis.KeyDeliveryQuota(provider, workspaceID)
	raw := c.client.Raw()

	// INCR returns the new count after incrementing.
	count, err := raw.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}

	// On first increment within a new window, set the TTL so the counter
	// auto-expires when the window elapses without further requests.
	if count == 1 {
		if err := raw.Expire(ctx, key, c.quotaTTL).Err(); err != nil {
			return false, 0, err
		}
	}

	allowed = count <= max
	remaining = max - count
	if remaining < 0 {
		remaining = 0
	}
	return allowed, remaining, nil
}

// AcquireMessageLock acquires a distributed lock for the given message ID.
// It returns the lock token and whether the lock was successfully acquired.
func (c *Cache) AcquireMessageLock(ctx context.Context, messageID string, ttl time.Duration) (token string, acquired bool, err error) {
	key := platformredis.KeyLockMessage(messageID)
	return c.client.AcquireLock(ctx, key, ttl)
}

// ReleaseMessageLock releases a previously acquired distributed lock for the
// given message ID. It returns true if the lock was successfully released.
func (c *Cache) ReleaseMessageLock(ctx context.Context, messageID, token string) (bool, error) {
	key := platformredis.KeyLockMessage(messageID)
	return c.client.ReleaseLock(ctx, key, token)
}
