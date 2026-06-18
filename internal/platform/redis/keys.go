package redis

import "fmt"

const (
	// keyVersion is the current key schema version. Bump to invalidate all
	// cached entries when payload shapes change in a backward-incompatible way.
	keyVersion = "v1"
)

// Key builders for cache-aside read acceleration. Every key is namespaced by
// module, versioned, and scoped to workspace where the data is tenant-specific.

func KeyIdentityWorkspaceAccess(workspaceID, userID string) string {
	return fmt.Sprintf("cache:identity:workspace-access:%s:%s:%s", keyVersion, workspaceID, userID)
}

func KeyIdentityWorkspaceSettings(workspaceID string) string {
	return fmt.Sprintf("cache:identity:workspace-settings:%s:%s", keyVersion, workspaceID)
}

func KeyContentTemplatePreview(workspaceID, templateID, payloadHash string) string {
	return fmt.Sprintf("cache:content:template-preview:%s:%s:%s:%s", keyVersion, workspaceID, templateID, payloadHash)
}

func KeySenderDomainAuth(workspaceID, senderDomainID string) string {
	return fmt.Sprintf("cache:sender:domain-auth:%s:%s:%s", keyVersion, workspaceID, senderDomainID)
}

func KeyAudienceSegmentMembership(workspaceID, segmentID string) string {
	return fmt.Sprintf("cache:audience:segment-membership:%s:%s:%s", keyVersion, workspaceID, segmentID)
}

func KeyDeliveryIdempotency(workspaceID, idempotencyKey string) string {
	return fmt.Sprintf("idem:delivery:transactional:%s:%s:%s", keyVersion, workspaceID, idempotencyKey)
}

func KeyDeliveryQuota(provider, workspaceID string) string {
	return fmt.Sprintf("quota:delivery:provider:%s:%s:%s", keyVersion, provider, workspaceID)
}

func KeyAPIKeyQuotaLimits(workspaceID, apiKeyID string) string {
	return fmt.Sprintf("cache:access:quota-limits:%s:%s:%s", keyVersion, workspaceID, apiKeyID)
}

func KeyAPIKeyQuotaBucket(workspaceID, apiKeyID, window string) string {
	return fmt.Sprintf("quota:apikey:token_bucket:%s:%s:%s:%s", keyVersion, workspaceID, apiKeyID, window)
}

func KeyLockMessage(messageID string) string {
	return fmt.Sprintf("lock:delivery:message:%s:%s", keyVersion, messageID)
}

func KeyCacheLoadLock(cacheKey string) string {
	return "lock:" + cacheKey
}

// Prefix helpers for workspace-scoped invalidation.

func PrefixIdentityWorkspaceAccess(workspaceID string) string {
	return fmt.Sprintf("cache:identity:workspace-access:%s:%s:", keyVersion, workspaceID)
}

func PrefixIdentityWorkspaceSettings(workspaceID string) string {
	return fmt.Sprintf("cache:identity:workspace-settings:%s:%s:", keyVersion, workspaceID)
}

func PrefixContentTemplatePreview(workspaceID, templateID string) string {
	return fmt.Sprintf("cache:content:template-preview:%s:%s:%s:", keyVersion, workspaceID, templateID)
}

func PrefixDeliveryIdempotency(workspaceID string) string {
	return fmt.Sprintf("idem:delivery:transactional:%s:%s", keyVersion, workspaceID)
}
