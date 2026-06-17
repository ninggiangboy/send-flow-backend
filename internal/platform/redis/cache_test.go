package redis

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

type cacheTestMetrics struct {
	cacheHits     atomic.Int64
	cacheMisses   atomic.Int64
	loadSuccesses atomic.Int64
	loadErrors    atomic.Int64
	setErrors     atomic.Int64
	deleteErrors  atomic.Int64
	lockAcquired  atomic.Int64
	lockContended atomic.Int64
	lockErrors    atomic.Int64
	loadObserved  atomic.Int64
	lockObserved  atomic.Int64
}

func (m *cacheTestMetrics) RecordCacheHit(module, resource string)         { m.cacheHits.Add(1) }
func (m *cacheTestMetrics) RecordCacheMiss(module, resource string)        { m.cacheMisses.Add(1) }
func (m *cacheTestMetrics) RecordCacheLoadSuccess(module, resource string) { m.loadSuccesses.Add(1) }
func (m *cacheTestMetrics) RecordCacheLoadError(module, resource string)   { m.loadErrors.Add(1) }
func (m *cacheTestMetrics) RecordCacheSetError(module, resource string)    { m.setErrors.Add(1) }
func (m *cacheTestMetrics) RecordCacheDeleteError(module, resource string) { m.deleteErrors.Add(1) }
func (m *cacheTestMetrics) RecordLockAcquired(module, resource string)     { m.lockAcquired.Add(1) }
func (m *cacheTestMetrics) RecordLockContended(module, resource string)    { m.lockContended.Add(1) }
func (m *cacheTestMetrics) RecordLockError(module, resource string)        { m.lockErrors.Add(1) }
func (m *cacheTestMetrics) ObserveLoadDuration(module, resource string, seconds float64) {
	m.loadObserved.Add(1)
}
func (m *cacheTestMetrics) ObserveLockDuration(module, resource string, seconds float64) {
	m.lockObserved.Add(1)
}

func newTestRedisClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	client := &Client{
		client: goredis.NewClient(&goredis.Options{
			Addr: server.Addr(),
		}),
	}
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return client, server
}

func TestCacheAsideGetOrLoadJSONHit(t *testing.T) {
	t.Parallel()

	client, _ := newTestRedisClient(t)
	metrics := &cacheTestMetrics{}
	aside := NewCacheAside(client, metrics)

	ctx := context.Background()
	key := KeyIdentityWorkspaceSettings("ws1")
	if err := client.SetJSON(ctx, key, map[string]string{"value": "cached"}, time.Minute); err != nil {
		t.Fatalf("set cache: %v", err)
	}

	var loaded atomic.Int64
	var dst map[string]string
	err := aside.GetOrLoadJSON(ctx, "identity", "workspace_settings", key, &dst, time.Minute, func() (any, error) {
		loaded.Add(1)
		return map[string]string{"value": "fresh"}, nil
	})
	if err != nil {
		t.Fatalf("GetOrLoadJSON returned error: %v", err)
	}
	if loaded.Load() != 0 {
		t.Fatalf("expected loader not to run, ran %d times", loaded.Load())
	}
	if dst["value"] != "cached" {
		t.Fatalf("got %q, want cached", dst["value"])
	}
	if metrics.cacheHits.Load() != 1 {
		t.Fatalf("expected 1 cache hit, got %d", metrics.cacheHits.Load())
	}
}

func TestCacheAsideGetOrLoadJSONSingleflight(t *testing.T) {
	t.Parallel()

	client, _ := newTestRedisClient(t)
	metrics := &cacheTestMetrics{}
	aside := NewCacheAside(client, metrics)

	ctx := context.Background()
	key := KeyIdentityWorkspaceSettings("ws1")
	var loadCalls atomic.Int64

	start := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]map[string]string, 8)
	errs := make([]error, 8)

	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			var dst map[string]string
			errs[i] = aside.GetOrLoadJSON(ctx, "identity", "workspace_settings", key, &dst, time.Minute, func() (any, error) {
				loadCalls.Add(1)
				<-release
				return map[string]string{"value": "fresh"}, nil
			})
			results[i] = dst
		}(i)
	}

	close(start)
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	if loadCalls.Load() != 1 {
		t.Fatalf("expected loader to run once, ran %d times", loadCalls.Load())
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d returned error: %v", i, err)
		}
		if results[i]["value"] != "fresh" {
			t.Fatalf("call %d got %q, want fresh", i, results[i]["value"])
		}
	}
	if metrics.lockAcquired.Load() != 1 {
		t.Fatalf("expected 1 acquired lock, got %d", metrics.lockAcquired.Load())
	}
	if metrics.loadSuccesses.Load() != 1 {
		t.Fatalf("expected 1 load success, got %d", metrics.loadSuccesses.Load())
	}
}

func TestCacheAsideGetOrLoadJSONReturnsValuePopulatedByOtherNode(t *testing.T) {
	t.Parallel()

	client, _ := newTestRedisClient(t)
	metrics := &cacheTestMetrics{}
	aside := NewCacheAside(client, metrics)
	aside.waitRetries = 3
	aside.waitBaseDelay = 0
	aside.waitJitterMax = 0

	ctx := context.Background()
	key := KeyIdentityWorkspaceSettings("ws1")
	lockKey := KeyCacheLoadLock(key)
	token, acquired, err := client.AcquireLock(ctx, lockKey, time.Minute)
	if err != nil {
		t.Fatalf("AcquireLock returned error: %v", err)
	}
	if !acquired {
		t.Fatal("expected initial lock acquisition to succeed")
	}
	defer func() {
		_, _ = client.ReleaseLock(context.Background(), lockKey, token)
	}()

	var slept atomic.Int64
	aside.sleep = func(ctx context.Context, d time.Duration) error {
		if slept.Add(1) == 1 {
			if err := client.SetJSON(ctx, key, map[string]string{"value": "from-other-node"}, time.Minute); err != nil {
				return err
			}
			_, _ = client.ReleaseLock(context.Background(), lockKey, token)
		}
		return nil
	}

	var loadCalls atomic.Int64
	var dst map[string]string
	err = aside.GetOrLoadJSON(ctx, "identity", "workspace_settings", key, &dst, time.Minute, func() (any, error) {
		loadCalls.Add(1)
		return map[string]string{"value": "fresh"}, nil
	})
	if err != nil {
		t.Fatalf("GetOrLoadJSON returned error: %v", err)
	}
	if loadCalls.Load() != 0 {
		t.Fatalf("expected loader not to run, ran %d times", loadCalls.Load())
	}
	if dst["value"] != "from-other-node" {
		t.Fatalf("got %q, want from-other-node", dst["value"])
	}
	if metrics.lockContended.Load() != 1 {
		t.Fatalf("expected 1 contended lock, got %d", metrics.lockContended.Load())
	}
}

func TestCacheAsideGetOrLoadJSONFallsBackAfterWait(t *testing.T) {
	t.Parallel()

	client, _ := newTestRedisClient(t)
	metrics := &cacheTestMetrics{}
	aside := NewCacheAside(client, metrics)
	aside.waitRetries = 2
	aside.waitBaseDelay = 0
	aside.waitJitterMax = 0
	aside.sleep = func(ctx context.Context, d time.Duration) error { return nil }

	ctx := context.Background()
	key := KeyIdentityWorkspaceSettings("ws1")
	lockKey := KeyCacheLoadLock(key)
	token, acquired, err := client.AcquireLock(ctx, lockKey, time.Minute)
	if err != nil {
		t.Fatalf("AcquireLock returned error: %v", err)
	}
	if !acquired {
		t.Fatal("expected initial lock acquisition to succeed")
	}
	defer func() {
		_, _ = client.ReleaseLock(context.Background(), lockKey, token)
	}()

	var loadCalls atomic.Int64
	var dst map[string]string
	err = aside.GetOrLoadJSON(ctx, "identity", "workspace_settings", key, &dst, time.Minute, func() (any, error) {
		loadCalls.Add(1)
		return map[string]string{"value": "fallback"}, nil
	})
	if err != nil {
		t.Fatalf("GetOrLoadJSON returned error: %v", err)
	}
	if loadCalls.Load() != 1 {
		t.Fatalf("expected loader to run once, ran %d times", loadCalls.Load())
	}
	if dst["value"] != "fallback" {
		t.Fatalf("got %q, want fallback", dst["value"])
	}

	var cached map[string]string
	if err := client.GetJSON(ctx, key, &cached); err != nil {
		t.Fatalf("expected fallback value to be cached: %v", err)
	}
	if cached["value"] != "fallback" {
		t.Fatalf("cached value = %q, want fallback", cached["value"])
	}
	if metrics.lockContended.Load() != 1 {
		t.Fatalf("expected 1 contended lock, got %d", metrics.lockContended.Load())
	}
}

func TestReleaseLockRequiresMatchingToken(t *testing.T) {
	t.Parallel()

	client, _ := newTestRedisClient(t)
	ctx := context.Background()
	key := KeyCacheLoadLock("cache:test")

	token, acquired, err := client.AcquireLock(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("AcquireLock returned error: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock acquisition to succeed")
	}

	released, err := client.ReleaseLock(ctx, key, "wrong-token")
	if err != nil {
		t.Fatalf("ReleaseLock returned error: %v", err)
	}
	if released {
		t.Fatal("expected wrong token not to release lock")
	}

	released, err = client.ReleaseLock(ctx, key, token)
	if err != nil {
		t.Fatalf("ReleaseLock returned error: %v", err)
	}
	if !released {
		t.Fatal("expected correct token to release lock")
	}
}
