package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
	goredis "github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("redis cache miss")

type Client struct {
	client *goredis.Client
}

func New(ctx context.Context, cfg config.Config) (*Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Client{client: client}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) Raw() *goredis.Client {
	return c.client
}

func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("redis json ttl must be positive")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal redis json value: %w", err)
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *Client) GetJSON(ctx context.Context, key string, dst any) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return ErrCacheMiss
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("unmarshal redis json value: %w", err)
	}
	return nil
}

func (c *Client) AcquireLock(ctx context.Context, key string, ttl time.Duration) (string, bool, error) {
	if ttl <= 0 {
		return "", false, errors.New("redis lock ttl must be positive")
	}
	token, err := randomToken(16)
	if err != nil {
		return "", false, err
	}
	ok, err := c.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return "", false, err
	}
	return token, ok, nil
}

func (c *Client) ReleaseLock(ctx context.Context, key, token string) (bool, error) {
	const script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`
	deleted, err := c.client.Eval(ctx, script, []string{key}, token).Int()
	if err != nil {
		return false, err
	}
	return deleted == 1, nil
}

func (c *Client) DeletePrefix(ctx context.Context, prefix string, batchSize int64) (int64, error) {
	if prefix == "" {
		return 0, errors.New("redis prefix is required")
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	var cursor uint64
	var deleted int64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, prefix+"*", batchSize).Result()
		if err != nil {
			return deleted, err
		}
		cursor = next
		if len(keys) > 0 {
			n, err := c.client.Del(ctx, keys...).Result()
			if err != nil {
				return deleted, err
			}
			deleted += n
		}
		if cursor == 0 {
			return deleted, nil
		}
	}
}

func randomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate redis lock token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
