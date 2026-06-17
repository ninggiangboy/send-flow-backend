package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/sync/singleflight"
)

var ErrDecodeError = errors.New("redis: decode error")

const (
	defaultCacheLoadLockTTL   = 5 * time.Second
	defaultCacheWaitRetries   = 4
	defaultCacheWaitBaseDelay = 25 * time.Millisecond
	defaultCacheWaitJitterMax = 25 * time.Millisecond
)

// CacheAside contains shared helpers for cache-aside read acceleration.
type CacheAside struct {
	client        *Client
	sf            singleflight.Group
	metrics       CacheMetrics
	lockTTL       time.Duration
	waitRetries   int
	waitBaseDelay time.Duration
	waitJitterMax time.Duration
	sleep         func(context.Context, time.Duration) error
	now           func() time.Time
}

type CacheMetrics interface {
	RecordCacheHit(module, resource string)
	RecordCacheMiss(module, resource string)
	RecordCacheLoadSuccess(module, resource string)
	RecordCacheLoadError(module, resource string)
	RecordCacheSetError(module, resource string)
	RecordCacheDeleteError(module, resource string)
	RecordLockAcquired(module, resource string)
	RecordLockContended(module, resource string)
	RecordLockError(module, resource string)
	ObserveLoadDuration(module, resource string, seconds float64)
	ObserveLockDuration(module, resource string, seconds float64)
}

type noopCacheMetrics struct{}

func (noopCacheMetrics) RecordCacheHit(module, resource string)         {}
func (noopCacheMetrics) RecordCacheMiss(module, resource string)        {}
func (noopCacheMetrics) RecordCacheLoadSuccess(module, resource string) {}
func (noopCacheMetrics) RecordCacheLoadError(module, resource string)   {}
func (noopCacheMetrics) RecordCacheSetError(module, resource string)    {}
func (noopCacheMetrics) RecordCacheDeleteError(module, resource string) {}
func (noopCacheMetrics) RecordLockAcquired(module, resource string)     {}
func (noopCacheMetrics) RecordLockContended(module, resource string)    {}
func (noopCacheMetrics) RecordLockError(module, resource string)        {}
func (noopCacheMetrics) ObserveLoadDuration(module, resource string, seconds float64) {
}
func (noopCacheMetrics) ObserveLockDuration(module, resource string, seconds float64) {
}

func NewCacheAside(client *Client, metrics CacheMetrics) *CacheAside {
	if metrics == nil {
		metrics = noopCacheMetrics{}
	}
	return &CacheAside{
		client:        client,
		metrics:       metrics,
		lockTTL:       defaultCacheLoadLockTTL,
		waitRetries:   defaultCacheWaitRetries,
		waitBaseDelay: defaultCacheWaitBaseDelay,
		waitJitterMax: defaultCacheWaitJitterMax,
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		now: time.Now,
	}
}

// GetOrLoadJSON implements the cache-aside pattern with in-process singleflight
// deduplication. On miss it calls loadFn, stores the result in Redis, and returns
// it. If loadFn returns an error the miss is NOT cached.
func (c *CacheAside) GetOrLoadJSON(ctx context.Context, module, resource, key string, dst any, ttl time.Duration, loadFn func() (any, error)) error {
	if err := c.client.GetJSON(ctx, key, dst); err == nil {
		c.metrics.RecordCacheHit(module, resource)
		return nil
	} else if !errors.Is(err, ErrCacheMiss) {
		return err
	}
	c.metrics.RecordCacheMiss(module, resource)

	val, err, _ := c.sf.Do(key, func() (any, error) {
		cached, cacheErr := c.readRawCachedValue(ctx, key)
		if cacheErr == nil {
			c.metrics.RecordCacheHit(module, resource)
			return cached, nil
		}
		if cacheErr != nil && !errors.Is(cacheErr, ErrCacheMiss) {
			return nil, cacheErr
		}

		lockKey := KeyCacheLoadLock(key)
		lockToken, locked, lockErr := c.client.AcquireLock(ctx, lockKey, c.lockTTL)
		if lockErr != nil {
			c.metrics.RecordLockError(module, resource)
			return c.loadAndCache(ctx, module, resource, key, ttl, loadFn)
		}
		if !locked {
			c.metrics.RecordLockContended(module, resource)
			waited, waitErr := c.waitForCachedValue(ctx, key)
			if waitErr == nil {
				c.metrics.RecordCacheHit(module, resource)
				return waited, nil
			}
			if waitErr != nil && !errors.Is(waitErr, ErrCacheMiss) {
				return nil, waitErr
			}
			return c.loadAndCache(ctx, module, resource, key, ttl, loadFn)
		}

		c.metrics.RecordLockAcquired(module, resource)
		lockStartedAt := c.now()
		defer func() {
			c.metrics.ObserveLockDuration(module, resource, c.now().Sub(lockStartedAt).Seconds())
			released, releaseErr := c.client.ReleaseLock(ctx, lockKey, lockToken)
			if releaseErr != nil || !released {
				c.metrics.RecordLockError(module, resource)
			}
		}()

		cached, cacheErr = c.readRawCachedValue(ctx, key)
		if cacheErr == nil {
			c.metrics.RecordCacheHit(module, resource)
			return cached, nil
		}
		if cacheErr != nil && !errors.Is(cacheErr, ErrCacheMiss) {
			return nil, cacheErr
		}

		return c.loadAndCache(ctx, module, resource, key, ttl, loadFn)
	})
	if err != nil {
		return err
	}

	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDecodeError, err)
	}
	return json.Unmarshal(data, dst)
}

// SetJSON delegates to the underlying client's SetJSON.
func (c *CacheAside) SetJSON(ctx context.Context, module, resource, key string, value any, ttl time.Duration) error {
	err := c.client.SetJSON(ctx, key, value, ttl)
	if err != nil {
		c.metrics.RecordCacheSetError(module, resource)
	}
	return err
}

// DeleteKey removes a single key from Redis.
func (c *CacheAside) DeleteKey(ctx context.Context, module, resource, key string) error {
	_, err := c.client.Delete(ctx, key)
	if err != nil {
		c.metrics.RecordCacheDeleteError(module, resource)
	}
	return err
}

// DeleteKeys removes multiple keys in a single round trip.
func (c *CacheAside) DeleteKeys(ctx context.Context, module, resource string, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := c.client.Delete(ctx, keys...)
	if err != nil {
		c.metrics.RecordCacheDeleteError(module, resource)
	}
	return err
}

// DeletePrefix removes all keys matching the given prefix using SCAN + DEL.
func (c *CacheAside) DeletePrefix(ctx context.Context, module, resource, prefix string, batchSize int64) (int64, error) {
	deleted, err := c.client.DeletePrefix(ctx, prefix, batchSize)
	if err != nil {
		c.metrics.RecordCacheDeleteError(module, resource)
	}
	return deleted, err
}

// IsCacheMiss reports whether err is a cache miss.
func IsCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss)
}

// IsDecodeError reports whether err is a decode error.
func IsDecodeError(err error) bool {
	return errors.Is(err, ErrDecodeError)
}

func (c *CacheAside) loadAndCache(
	ctx context.Context,
	module, resource, key string,
	ttl time.Duration,
	loadFn func() (any, error),
) (any, error) {
	startedAt := c.now()
	data, loadErr := loadFn()
	c.metrics.ObserveLoadDuration(module, resource, c.now().Sub(startedAt).Seconds())
	if loadErr != nil {
		c.metrics.RecordCacheLoadError(module, resource)
		return nil, loadErr
	}
	c.metrics.RecordCacheLoadSuccess(module, resource)
	if setErr := c.client.SetJSON(ctx, key, data, ttl); setErr != nil {
		c.metrics.RecordCacheSetError(module, resource)
	}
	return data, nil
}

func (c *CacheAside) readRawCachedValue(ctx context.Context, key string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.client.GetJSON(ctx, key, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *CacheAside) waitForCachedValue(ctx context.Context, key string) (json.RawMessage, error) {
	for attempt := 0; attempt < c.waitRetries; attempt++ {
		if err := c.sleep(ctx, c.waitDelay(attempt)); err != nil {
			return nil, err
		}
		cached, err := c.readRawCachedValue(ctx, key)
		if err == nil {
			return cached, nil
		}
		if !errors.Is(err, ErrCacheMiss) {
			return nil, err
		}
	}
	return nil, ErrCacheMiss
}

func (c *CacheAside) waitDelay(attempt int) time.Duration {
	delay := c.waitBaseDelay * time.Duration(attempt+1)
	if c.waitJitterMax <= 0 {
		return delay
	}
	jitter := time.Duration(rand.Int63n(c.waitJitterMax.Nanoseconds() + 1))
	return delay + jitter
}
