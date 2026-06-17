package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

func newTestCacheAside(t *testing.T) (*platformredis.CacheAside, *platformredis.Client, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	client, err := platformredis.New(context.Background(), config.Config{RedisAddr: server.Addr()})
	if err != nil {
		t.Fatalf("platformredis.New returned error: %v", err)
	}
	aside := platformredis.NewCacheAside(client, nil)
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return aside, client, server
}

func TestInvalidateWorkspaceRemovesAnalyticsCacheForWorkspace(t *testing.T) {
	t.Parallel()

	aside, client, _ := newTestCacheAside(t)
	cache := NewCache(aside, time.Minute)
	ctx := context.Background()

	keys := map[string]any{
		"cache:analytics:dashboard:v1:ws1":      map[string]string{"kind": "dashboard"},
		"cache:analytics:deliverability:v1:ws1": map[string]string{"kind": "deliverability"},
		"cache:analytics:campaign:v1:ws1:cmp1":  map[string]string{"kind": "campaign"},
		"cache:analytics:campaign:v1:ws2:cmp2":  map[string]string{"kind": "other-workspace"},
	}
	for key, value := range keys {
		if err := client.SetJSON(ctx, key, value, time.Minute); err != nil {
			t.Fatalf("SetJSON(%s): %v", key, err)
		}
	}

	if err := cache.InvalidateWorkspace(ctx, "ws1"); err != nil {
		t.Fatalf("InvalidateWorkspace returned error: %v", err)
	}

	for _, key := range []string{
		"cache:analytics:dashboard:v1:ws1",
		"cache:analytics:deliverability:v1:ws1",
		"cache:analytics:campaign:v1:ws1:cmp1",
	} {
		var dst map[string]string
		if err := client.GetJSON(ctx, key, &dst); err == nil {
			t.Fatalf("expected key %s to be deleted", key)
		}
	}

	var preserved map[string]string
	if err := client.GetJSON(ctx, "cache:analytics:campaign:v1:ws2:cmp2", &preserved); err != nil {
		t.Fatalf("expected other workspace key to remain: %v", err)
	}
}
