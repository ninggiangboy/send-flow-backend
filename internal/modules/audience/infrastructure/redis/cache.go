package redis

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

const (
	module   = "audience"
	resource = "segment_membership"

	maxCachedIDs = 10000

	selectionKeyFmt = "cache:audience:selection:v1:%s:%s:%s:%s"
	selectionPrefix = "cache:audience:selection:v1:%s:"
)

// Cache wraps *platformredis.CacheAside to provide audience resolution caching.
type Cache struct {
	aside *platformredis.CacheAside
	ttl   time.Duration
}

// NewCache returns a new Cache that wraps the given cache-aside backend.
func NewCache(aside *platformredis.CacheAside, ttl time.Duration) *Cache {
	return &Cache{aside: aside, ttl: ttl}
}

// GetOrLoadSelection retrieves the cached audience selection for the given
// workspace, list, segment, and contact IDs. On a cache miss it calls loadFn,
// stores the result, and returns it. If the result set exceeds maxCachedIDs the
// entry is not cached; subsequent calls will fall through to loadFn directly.
func (c *Cache) GetOrLoadSelection(
	ctx context.Context,
	workspaceID, listID, segmentID string,
	contactIDs []string,
	loadFn func() ([]string, error),
) ([]string, error) {
	key := buildSelectionKey(workspaceID, listID, segmentID, contactIDs)

	var result []string
	tooLarge := false

	err := c.aside.GetOrLoadJSON(ctx, module, resource, key, &result, c.ttl, func() (any, error) {
		data, loadErr := loadFn()
		if loadErr != nil {
			return nil, loadErr
		}
		if len(data) > maxCachedIDs {
			tooLarge = true
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}

	// If the loaded result exceeds the threshold, remove it from cache so
	// subsequent requests always call loadFn directly.
	if tooLarge {
		_ = c.aside.DeleteKey(ctx, module, resource, key)
	}

	return result, nil
}

// InvalidateSelection removes all cached selection entries scoped to the given
// workspace. Call this whenever contact membership changes for the workspace.
func (c *Cache) InvalidateSelection(ctx context.Context, workspaceID string) error {
	prefix := fmt.Sprintf(selectionPrefix, workspaceID)
	_, err := c.aside.DeletePrefix(ctx, module, resource, prefix, 100)
	return err
}

// buildSelectionKey returns a deterministic cache key for an audience selection.
func buildSelectionKey(workspaceID, listID, segmentID string, contactIDs []string) string {
	if len(contactIDs) == 0 {
		return platformredis.KeyAudienceSegmentMembership(workspaceID, segmentID)
	}

	sorted := make([]string, len(contactIDs))
	copy(sorted, contactIDs)
	sort.Strings(sorted)

	idPart := strings.Join(sorted, ",")
	return fmt.Sprintf(selectionKeyFmt, workspaceID, listID, segmentID, idPart)
}
