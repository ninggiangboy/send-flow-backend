package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

type OAuthStateStore struct{ client *redis.Client }

func NewOAuthStateStore(client *redis.Client) *OAuthStateStore {
	return &OAuthStateStore{client: client}
}

func (s *OAuthStateStore) Save(ctx context.Context, state string, value ports.OAuthState, ttl time.Duration) error {
	return s.client.SetJSON(ctx, fmt.Sprintf("identity:oauth:state:%s", state), value, ttl)
}

func (s *OAuthStateStore) GetAndDelete(ctx context.Context, state string) (*ports.OAuthState, error) {
	key := fmt.Sprintf("identity:oauth:state:%s", state)
	var v ports.OAuthState
	if err := s.client.GetJSON(ctx, key, &v); err != nil {
		return nil, err
	}
	_ = s.client.Raw().Del(ctx, key).Err()
	return &v, nil
}

type RefreshStore struct{ client *redis.Client }

func NewRefreshStore(client *redis.Client) *RefreshStore {
	return &RefreshStore{client: client}
}

func (s *RefreshStore) Save(ctx context.Context, refreshJTI, sessionID string, ttl time.Duration) error {
	return s.client.Raw().Set(ctx, fmt.Sprintf("identity:refresh:%s", refreshJTI), sessionID, ttl).Err()
}

func (s *RefreshStore) Find(ctx context.Context, refreshJTI string) (string, error) {
	value, err := s.client.Raw().Get(ctx, fmt.Sprintf("identity:refresh:%s", refreshJTI)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", ports.ErrCacheMiss
		}
		return "", err
	}
	return value, nil
}

func (s *RefreshStore) Delete(ctx context.Context, refreshJTI string) error {
	return s.client.Raw().Del(ctx, fmt.Sprintf("identity:refresh:%s", refreshJTI)).Err()
}

func (s *RefreshStore) Replace(ctx context.Context, oldRefreshJTI, newRefreshJTI, sessionID string, ttl time.Duration) error {
	pipe := s.client.Raw().TxPipeline()
	pipe.Del(ctx, fmt.Sprintf("identity:refresh:%s", oldRefreshJTI))
	pipe.Set(ctx, fmt.Sprintf("identity:refresh:%s", newRefreshJTI), sessionID, ttl)
	_, err := pipe.Exec(ctx)
	return err
}
