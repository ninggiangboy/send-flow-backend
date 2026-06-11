package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type Manager struct {
	pool *pgxpool.Pool
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

func (m *Manager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf("transaction manager is not configured")
	}
	return pgx.BeginFunc(ctx, m.pool, func(tx pgx.Tx) error {
		return fn(postgres.ContextWithTx(ctx, tx))
	})
}

var _ UnitOfWork = (*Manager)(nil)
