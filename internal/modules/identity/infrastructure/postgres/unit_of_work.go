package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type UnitOfWork struct {
	pool *pgxpool.Pool
}

func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

func (uow *UnitOfWork) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := uow.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	txCtx := WithTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Ensure UnitOfWork implements ports.UnitOfWork at compile time.
var _ ports.UnitOfWork = (*UnitOfWork)(nil)