package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Manager struct {
	pool *pgxpool.Pool
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

func (m *Manager) WithinTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("transaction manager is not configured")
	}
	return pgx.BeginFunc(ctx, m.pool, func(tx pgx.Tx) error {
		return fn(ctx, tx)
	})
}
