package ratelimit

import (
	"context"
	"fmt"
	"time"

	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

type Service interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

type RedisService struct {
	client *platformredis.Client
}

func NewRedisService(client *platformredis.Client) *RedisService {
	return &RedisService{client: client}
}

func (s *RedisService) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	pipe := s.client.Raw().TxPipeline()
	count := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return count.Val() <= limit, nil
}
