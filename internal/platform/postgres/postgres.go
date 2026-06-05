package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

type Client struct {
	writePool *pgxpool.Pool
	readPool  *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*Client, error) {
	writeCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	configurePool(writeCfg)

	writePool, err := pgxpool.NewWithConfig(ctx, writeCfg)
	if err != nil {
		return nil, err
	}
	readURL := cfg.DatabaseReadURL
	if readURL == "" {
		readURL = cfg.DatabaseURL
	}
	readCfg, err := pgxpool.ParseConfig(readURL)
	if err != nil {
		writePool.Close()
		return nil, err
	}
	configurePool(readCfg)
	readPool, err := pgxpool.NewWithConfig(ctx, readCfg)
	if err != nil {
		writePool.Close()
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := writePool.Ping(pingCtx); err != nil {
		writePool.Close()
		readPool.Close()
		return nil, err
	}
	if err := readPool.Ping(pingCtx); err != nil {
		writePool.Close()
		readPool.Close()
		return nil, err
	}
	return &Client{writePool: writePool, readPool: readPool}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.writePool.Ping(ctx)
}

func (c *Client) PingRead(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.readPool.Ping(ctx)
}

func (c *Client) Close() {
	if c.readPool != nil {
		c.readPool.Close()
	}
	if c.writePool != nil {
		c.writePool.Close()
	}
}

func (c *Client) Pool() *pgxpool.Pool {
	return c.writePool
}

func (c *Client) WritePool() *pgxpool.Pool {
	return c.writePool
}

func (c *Client) ReadPool() *pgxpool.Pool {
	return c.readPool
}

func configurePool(poolCfg *pgxpool.Config) {
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = 15 * time.Second
}
