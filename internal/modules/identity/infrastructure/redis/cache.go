package redis

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

const (
	module           = "identity"
	resourceAccess   = "workspace_access"
	resourceSettings = "workspace_settings"
)

type Cache struct {
	aside       *redis.CacheAside
	accessTTL   time.Duration
	settingsTTL time.Duration
}

func NewCache(aside *redis.CacheAside, accessTTL, settingsTTL time.Duration) *Cache {
	return &Cache{
		aside:       aside,
		accessTTL:   accessTTL,
		settingsTTL: settingsTTL,
	}
}

// --- Workspace Access ---

// GetOrLoadWorkspaceAccess reads from cache first; on miss it calls loadFn,
// stores the result, and returns it. Returns ErrCacheMiss if loadFn returns nil, nil.
func (c *Cache) GetOrLoadWorkspaceAccess(ctx context.Context, workspaceID, userID string, loadFn func() (*domain.Membership, error)) (*domain.Membership, error) {
	key := redis.KeyIdentityWorkspaceAccess(workspaceID, userID)
	var m domain.Membership
	err := c.aside.GetOrLoadJSON(ctx, module, resourceAccess, key, &m, c.accessTTL, func() (any, error) {
		data, err := loadFn()
		if err != nil {
			return nil, err
		}
		if data == nil {
			return nil, redis.ErrCacheMiss
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// InvalidateWorkspaceAccess removes a single workspace access entry.
func (c *Cache) InvalidateWorkspaceAccess(ctx context.Context, workspaceID, userID string) error {
	key := redis.KeyIdentityWorkspaceAccess(workspaceID, userID)
	return c.aside.DeleteKey(ctx, module, resourceAccess, key)
}

// InvalidateWorkspaceAccessByPrefix removes all workspace access entries for a workspace.
func (c *Cache) InvalidateWorkspaceAccessByPrefix(ctx context.Context, workspaceID string) error {
	prefix := redis.PrefixIdentityWorkspaceAccess(workspaceID)
	_, err := c.aside.DeletePrefix(ctx, module, resourceAccess, prefix, 100)
	return err
}

// --- Workspace Settings ---

// GetOrLoadWorkspaceSettings reads from cache first; on miss it calls loadFn,
// stores the result, and returns it. Returns ErrCacheMiss if loadFn returns nil, nil.
func (c *Cache) GetOrLoadWorkspaceSettings(ctx context.Context, workspaceID string, loadFn func() (*domain.WorkspaceSettings, error)) (*domain.WorkspaceSettings, error) {
	key := redis.KeyIdentityWorkspaceSettings(workspaceID)
	var s domain.WorkspaceSettings
	err := c.aside.GetOrLoadJSON(ctx, module, resourceSettings, key, &s, c.settingsTTL, func() (any, error) {
		data, err := loadFn()
		if err != nil {
			return nil, err
		}
		if data == nil {
			return nil, redis.ErrCacheMiss
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// InvalidateWorkspaceSettings removes the workspace settings cache entry.
func (c *Cache) InvalidateWorkspaceSettings(ctx context.Context, workspaceID string) error {
	key := redis.KeyIdentityWorkspaceSettings(workspaceID)
	return c.aside.DeleteKey(ctx, module, resourceSettings, key)
}
