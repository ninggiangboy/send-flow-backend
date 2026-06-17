package redis

import (
	"context"
	"time"

	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

type Cache struct {
	aside *platformredis.CacheAside
	ttl   time.Duration
}

func NewCache(aside *platformredis.CacheAside, ttl time.Duration) *Cache {
	return &Cache{aside: aside, ttl: ttl}
}

func (c *Cache) GetOrLoadDashboard(ctx context.Context, workspaceID string, loadFn func() (any, error)) (any, error) {
	key := "cache:analytics:dashboard:v1:" + workspaceID
	var result any
	err := c.aside.GetOrLoadJSON(ctx, "analytics", "dashboard", key, &result, c.ttl, loadFn)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Cache) GetOrLoadCampaign(ctx context.Context, workspaceID, campaignID string, loadFn func() (any, error)) (any, error) {
	key := "cache:analytics:campaign:v1:" + workspaceID + ":" + campaignID
	var result any
	err := c.aside.GetOrLoadJSON(ctx, "analytics", "campaign", key, &result, c.ttl, loadFn)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Cache) GetOrLoadDeliverability(ctx context.Context, workspaceID string, loadFn func() (any, error)) (any, error) {
	key := "cache:analytics:deliverability:v1:" + workspaceID
	var result any
	err := c.aside.GetOrLoadJSON(ctx, "analytics", "deliverability", key, &result, c.ttl, loadFn)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Cache) InvalidateWorkspace(ctx context.Context, workspaceID string) error {
	keys := []string{
		"cache:analytics:dashboard:v1:" + workspaceID,
		"cache:analytics:deliverability:v1:" + workspaceID,
	}
	if err := c.aside.DeleteKeys(ctx, "analytics", "cache", keys...); err != nil {
		return err
	}
	_, err := c.aside.DeletePrefix(ctx, "analytics", "cache", "cache:analytics:campaign:v1:"+workspaceID+":", 100)
	return err
}
