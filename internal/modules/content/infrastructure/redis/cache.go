package redis

import (
	"context"
	"time"

	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

// PreviewResult represents a rendered template preview.
type PreviewResult struct {
	Subject  string   `json:"subject"`
	HTML     string   `json:"html"`
	Text     string   `json:"text"`
	Warnings []string `json:"warnings,omitempty"`
}

// Cache provides content-related caching using the cache-aside pattern.
type Cache struct {
	aside *platformredis.CacheAside
	ttl   time.Duration
}

// NewCache returns a new Cache that wraps the given cache-aside backend.
func NewCache(aside *platformredis.CacheAside, ttl time.Duration) *Cache {
	return &Cache{aside: aside, ttl: ttl}
}

// GetOrLoadPreview retrieves a cached template preview by workspace, template,
// and payload hash. On a cache miss it calls loadFn, stores the result, and
// returns it. If loadFn returns an error the miss is not cached.
func (c *Cache) GetOrLoadPreview(
	ctx context.Context,
	workspaceID, templateID, payloadHash string,
	loadFn func() (*PreviewResult, error),
) (*PreviewResult, error) {
	key := platformredis.KeyContentTemplatePreview(workspaceID, templateID, payloadHash)
	var result PreviewResult
	err := c.aside.GetOrLoadJSON(ctx, "content", "template_preview", key, &result, c.ttl, func() (any, error) {
		return loadFn()
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// InvalidatePreview deletes all cached previews for a given workspace and
// template. This should be called whenever the template content changes.
func (c *Cache) InvalidatePreview(ctx context.Context, workspaceID, templateID string) error {
	prefix := platformredis.PrefixContentTemplatePreview(workspaceID, templateID)
	_, err := c.aside.DeletePrefix(ctx, "content", "template_preview", prefix, 10)
	return err
}
