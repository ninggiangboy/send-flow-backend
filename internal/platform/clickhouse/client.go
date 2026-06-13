package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

type Client struct {
	conn driver.Conn
}

type Option func(*Client)

func WithMetrics(metrics *observability.ClickHouseMetrics, queryType string) Option {
	return func(c *Client) {
		c.conn = observability.NewMetricsConn(c.conn, metrics, queryType)
	}
}

func New(ctx context.Context, dsn string, opts ...Option) (*Client, error) {
	chOpts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}

	conn, err := clickhouse.Open(chOpts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	c := &Client{conn: conn}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.conn.Ping(ctx)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Conn() driver.Conn {
	return c.conn
}
