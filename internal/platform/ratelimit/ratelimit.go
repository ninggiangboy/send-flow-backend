package ratelimit

import (
	"context"
	"fmt"
	"time"
)

const incrWithTTLLua = `
local c = redis.call('INCR', KEYS[1])
if c == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return c
`

type RateLimiterClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

type Service interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

type RedisService struct {
	client RateLimiterClient
}

func NewRedisService(client RateLimiterClient) *RedisService {
	return &RedisService{client: client}
}

func (s *RedisService) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	ttl := int64(window.Seconds())
	if ttl <= 0 {
		ttl = 1
	}
	result, err := s.client.Eval(ctx, incrWithTTLLua, []string{redisKey}, ttl)
	if err != nil {
		return false, err
	}
	count, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("ratelimit: unexpected Eval result type: %T", result)
	}
	return count <= limit, nil
}
