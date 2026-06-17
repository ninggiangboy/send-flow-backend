package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

type deleteMetricSpy struct {
	module   string
	resource string
	count    int
}

func (s *deleteMetricSpy) RecordCacheHit(module, resource string)         {}
func (s *deleteMetricSpy) RecordCacheMiss(module, resource string)        {}
func (s *deleteMetricSpy) RecordCacheLoadSuccess(module, resource string) {}
func (s *deleteMetricSpy) RecordCacheLoadError(module, resource string)   {}
func (s *deleteMetricSpy) RecordCacheSetError(module, resource string)    {}
func (s *deleteMetricSpy) RecordLockAcquired(module, resource string)     {}
func (s *deleteMetricSpy) RecordLockContended(module, resource string)    {}
func (s *deleteMetricSpy) RecordLockError(module, resource string)        {}
func (s *deleteMetricSpy) ObserveLoadDuration(module, resource string, seconds float64) {
}
func (s *deleteMetricSpy) ObserveLockDuration(module, resource string, seconds float64) {
}
func (s *deleteMetricSpy) RecordCacheDeleteError(module, resource string) {
	s.module = module
	s.resource = resource
	s.count++
}

func newTestCacheAside(t *testing.T, metrics platformredis.CacheMetrics) (*platformredis.CacheAside, *platformredis.Client, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	client, err := platformredis.New(context.Background(), config.Config{RedisAddr: server.Addr()})
	if err != nil {
		t.Fatalf("platformredis.New returned error: %v", err)
	}
	aside := platformredis.NewCacheAside(client, metrics)
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return aside, client, server
}

func TestInvalidateWorkspaceSettingsDeletesKey(t *testing.T) {
	t.Parallel()

	aside, client, _ := newTestCacheAside(t, nil)
	cache := NewCache(aside, time.Minute, time.Minute)
	ctx := context.Background()
	key := platformredis.KeyIdentityWorkspaceSettings("ws1")

	if err := client.SetJSON(ctx, key, map[string]string{"theme": "dark"}, time.Minute); err != nil {
		t.Fatalf("SetJSON returned error: %v", err)
	}

	if err := cache.InvalidateWorkspaceSettings(ctx, "ws1"); err != nil {
		t.Fatalf("InvalidateWorkspaceSettings returned error: %v", err)
	}

	var dst map[string]string
	if err := client.GetJSON(ctx, key, &dst); err == nil {
		t.Fatal("expected workspace settings key to be deleted")
	}
}

func TestInvalidateWorkspaceSettingsRecordsCorrectResourceOnDeleteError(t *testing.T) {
	t.Parallel()

	metrics := &deleteMetricSpy{}
	aside, client, server := newTestCacheAside(t, metrics)
	cache := NewCache(aside, time.Minute, time.Minute)
	ctx := context.Background()

	server.Close()
	_ = client.Close()

	if err := cache.InvalidateWorkspaceSettings(ctx, "ws1"); err == nil {
		t.Fatal("expected delete error after closing redis client")
	}
	if metrics.count != 1 {
		t.Fatalf("expected 1 delete error metric, got %d", metrics.count)
	}
	if metrics.module != module {
		t.Fatalf("got module %q, want %q", metrics.module, module)
	}
	if metrics.resource != resourceSettings {
		t.Fatalf("got resource %q, want %q", metrics.resource, resourceSettings)
	}
}
