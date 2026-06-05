package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type txKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func TxFromCtx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

func (r *UserWriteRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *UserReadRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *ExternalAccountWriteRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *ExternalAccountReadRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *SessionWriteRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *SessionReadRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *AuthTokenRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *TOTPRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *WorkspaceWriteRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *WorkspaceReadRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *RoleWriteRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *RoleReadRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *MembershipWriteRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *MembershipReadRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *InvitationWriteRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *InvitationReadRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := TxFromCtx(ctx); ok {
		return tx
	}
	return r.db
}