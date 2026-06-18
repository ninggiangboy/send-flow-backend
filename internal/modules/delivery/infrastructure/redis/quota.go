package redis

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"

	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	deliverydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

// tokenBucketLua is an atomic Lua script that checks and consumes tokens across
// one or more quota windows. Phase 1 validates ALL windows without modifying
// state; Phase 2 commits deductions only if all windows passed.
//
// KEYS: bucket keys for each configured window (1 to 4, in order).
// ARGV: [now_unix, units, cap1, win_sec1, cap2, win_sec2, ...]
//
// Returns {1, 0} on success, {0, failed_index} on failure.
const tokenBucketLua = `
local now = tonumber(ARGV[1])
local units = tonumber(ARGV[2])
local n = #KEYS

-- Phase 1: check all windows, compute new values
local states = {}
for i = 1, n do
    local capacity = tonumber(ARGV[2 + (i-1)*2 + 1])
    local window_sec = tonumber(ARGV[2 + (i-1)*2 + 2])

    local data = redis.call('HMGET', KEYS[i], 'tokens', 'last_refill_at')
    local tokens = tonumber(data[1])
    local last_refill = tonumber(data[2])

    if tokens == nil then
        tokens = capacity
        last_refill = now
    end

    -- continuous refill based on elapsed real time
    local elapsed = math.max(0, now - last_refill)
    local refill = elapsed * (capacity / window_sec)
    tokens = math.min(capacity, tokens + refill)

    if tokens < units then
        return {0, i}
    end

    states[i] = {tokens - units, math.ceil(2 * window_sec)}
end

-- Phase 2: all windows passed, commit
for i = 1, n do
    redis.call('HSET', KEYS[i], 'tokens', states[i][1], 'last_refill_at', now)
    redis.call('EXPIRE', KEYS[i], states[i][2])
end

return {1, 0}
`

// windowConfig maps an EmailQuotaLimits field name to its Redis key suffix
// and window duration in seconds.
type windowConfig struct {
	keySuffix string
	seconds   int64
}

var quotaWindows = []struct {
	field  func(*accessdomain.EmailQuotaLimits) *int
	config windowConfig
}{
	{func(l *accessdomain.EmailQuotaLimits) *int { return l.PerMinute }, windowConfig{keySuffix: "minute", seconds: 60}},
	{func(l *accessdomain.EmailQuotaLimits) *int { return l.PerHour }, windowConfig{keySuffix: "hour", seconds: 3600}},
	{func(l *accessdomain.EmailQuotaLimits) *int { return l.PerDay }, windowConfig{keySuffix: "day", seconds: 86400}},
	{func(l *accessdomain.EmailQuotaLimits) *int { return l.PerMonth }, windowConfig{keySuffix: "month", seconds: 2592000}}, // 30 days
}

// TokenBucketService provides atomic token bucket quota enforcement using Redis
// and Lua. Each API key+window pair is a Redis HASH storing current tokens and
// last refill timestamp. Tokens refill continuously based on elapsed real time.
type TokenBucketService struct {
	client *platformredis.Client
	log    *slog.Logger
}

// NewTokenBucketService creates a new TokenBucketService backed by the given
// Redis client.
func NewTokenBucketService(client *platformredis.Client, log *slog.Logger) *TokenBucketService {
	return &TokenBucketService{
		client: client,
		log:    log.With("component", "token_bucket"),
	}
}

// CheckAndConsume atomically checks and consumes tokens for all configured
// quota windows. Returns nil when within quota, ErrAPIKeyQuotaExceeded when
// any window is exhausted, or ErrTemporarilyUnavailable on Redis failure.
// When limits is nil or empty, no enforcement is applied.
func (s *TokenBucketService) CheckAndConsume(ctx context.Context, workspaceID, apiKeyID string, recipientCount int, limits *accessdomain.EmailQuotaLimits) error {
	if limits == nil || limits.IsEmpty() {
		return nil
	}

	now := time.Now().UTC().Unix()

	var keys []string
	var args []string

	args = append(args, strconv.FormatInt(now, 10))
	args = append(args, strconv.Itoa(recipientCount))

	for _, w := range quotaWindows {
		val := w.field(limits)
		if val == nil {
			continue
		}
		key := platformredis.KeyAPIKeyQuotaBucket(workspaceID, apiKeyID, w.config.keySuffix)
		keys = append(keys, key)
		args = append(args, strconv.Itoa(*val), strconv.FormatInt(w.config.seconds, 10))
	}

	if len(keys) == 0 {
		return nil
	}

	result, err := s.client.Eval(ctx, tokenBucketLua, keys, argsToAny(args)...)
	if err != nil {
		s.log.Error("redis token bucket eval failed", "error", err, "workspace_id", workspaceID)
		return deliverydomain.ErrTemporarilyUnavailable
	}

	arr, ok := result.([]interface{})
	if !ok || len(arr) < 2 {
		s.log.Error("unexpected token bucket result", "result", result)
		return deliverydomain.ErrAPIKeyQuotaExceeded
	}

	allowed, ok := arr[0].(int64)
	if !ok || allowed != 1 {
		// failedWindow, _ := arr[1].(int64) // available for logging
		return deliverydomain.ErrAPIKeyQuotaExceeded
	}

	return nil
}

func argsToAny(args []string) []any {
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = a
	}
	return out
}
