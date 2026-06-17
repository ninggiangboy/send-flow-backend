package redis

import (
	"context"
	"time"

	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

// Domain represents a sender domain configuration.
type Domain struct {
	ID                string `json:"id"`
	Domain            string `json:"domain"`
	Status            string `json:"status"`
	DNSVerified       bool   `json:"dns_verified"`
	DKIMVerified      bool   `json:"dkim_verified"`
	SPFVerified       bool   `json:"spf_verified"`
	TrackingVerified  bool   `json:"tracking_verified"`
}

// DNSRecord represents a DNS record for domain verification.
type DNSRecord struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

// SenderReadiness represents the readiness status of a sender domain,
// including the domain itself and the required DNS records.
type SenderReadiness struct {
	Domain     *Domain      `json:"domain"`
	DNSRecords []DNSRecord  `json:"dns_records"`
}

// Cache provides sender-related caching using the cache-aside pattern.
type Cache struct {
	aside *platformredis.CacheAside
	ttl   time.Duration
}

// NewCache returns a new Cache that wraps the given cache-aside backend.
func NewCache(aside *platformredis.CacheAside, ttl time.Duration) *Cache {
	return &Cache{aside: aside, ttl: ttl}
}

// GetOrLoadReadiness retrieves cached sender domain readiness by workspace and
// sender domain. On a cache miss it calls loadFn, stores the result, and
// returns it. If loadFn returns an error the miss is not cached.
func (c *Cache) GetOrLoadReadiness(
	ctx context.Context,
	workspaceID, senderDomainID string,
	loadFn func() (*SenderReadiness, error),
) (*SenderReadiness, error) {
	key := platformredis.KeySenderDomainAuth(workspaceID, senderDomainID)
	var result SenderReadiness
	err := c.aside.GetOrLoadJSON(ctx, "sender", "readiness", key, &result, c.ttl, func() (any, error) {
		return loadFn()
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// InvalidateReadiness deletes the cached readiness for a given sender domain.
// This should be called whenever the domain's verification status changes.
func (c *Cache) InvalidateReadiness(ctx context.Context, workspaceID, senderDomainID string) error {
	key := platformredis.KeySenderDomainAuth(workspaceID, senderDomainID)
	return c.aside.DeleteKey(ctx, "sender", "readiness", key)
}
