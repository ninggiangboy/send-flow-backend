package api

import (
	"context"
	"log/slog"
	"time"

	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	deliveryredis "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/redis"
	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

// apiKeyQuotaReader is the narrow interface the enforcer needs to resolve
// API key quota limits from Postgres (via the access module repository).
type apiKeyQuotaReader interface {
	FindByID(ctx context.Context, workspaceID, keyID string) (*accessdomain.APIKey, error)
}

// apiKeyQuotaEnforcer implements ports.QuotaEnforcer by reading quota config
// through a cache-aside pattern and delegating enforcement to the token bucket
// service.
type apiKeyQuotaEnforcer struct {
	bucket   *deliveryredis.TokenBucketService
	reader   apiKeyQuotaReader
	cache    *platformredis.CacheAside
	cacheTTL time.Duration
	log      *slog.Logger
}

func newAPIKeyQuotaEnforcer(
	bucket *deliveryredis.TokenBucketService,
	reader apiKeyQuotaReader,
	cache *platformredis.CacheAside,
	cacheTTL time.Duration,
	log *slog.Logger,
) *apiKeyQuotaEnforcer {
	return &apiKeyQuotaEnforcer{
		bucket:   bucket,
		reader:   reader,
		cache:    cache,
		cacheTTL: cacheTTL,
		log:      log.With("component", "quota_enforcer"),
	}
}

// CheckAndConsume resolves the API key quota limits (from cache or Postgres)
// and delegates to the token bucket service for enforcement.
func (e *apiKeyQuotaEnforcer) CheckAndConsume(ctx context.Context, workspaceID, apiKeyID string, recipientCount int) error {
	cacheKey := platformredis.KeyAPIKeyQuotaLimits(workspaceID, apiKeyID)

	var limits *accessdomain.EmailQuotaLimits
	err := e.cache.GetOrLoadJSON(ctx, "access", "quota_limits", cacheKey, &limits, e.cacheTTL, func() (any, error) {
		key, err := e.reader.FindByID(ctx, workspaceID, apiKeyID)
		if err != nil {
			return nil, err
		}
		return key.QuotaLimits, nil
	})
	if err != nil {
		e.log.Error("failed to resolve api key quota limits",
			"workspace_id", workspaceID,
			"api_key_id", apiKeyID,
			"error", err,
		)
		// Fail closed: treat resolution error as temporary unavailability
		return err
	}

	return e.bucket.CheckAndConsume(ctx, workspaceID, apiKeyID, recipientCount, limits)
}
