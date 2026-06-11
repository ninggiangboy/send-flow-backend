package postgres

import (
	"context"

	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

func getDB(ctx context.Context, db platformpostgres.DBTX) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return db
}

func (r *UserWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *UserReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *ExternalAccountWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *ExternalAccountReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *SessionWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *SessionReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *AuthTokenRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *TOTPRepository) getDB(ctx context.Context) platformpostgres.DBTX { return getDB(ctx, r.db) }
func (r *WorkspaceWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *WorkspaceReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *RoleWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *RoleReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *MembershipWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *MembershipReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *InvitationWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *InvitationReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *SettingsReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *SettingsWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	return getDB(ctx, r.db)
}
func (r *OutboxRepository) getDB(ctx context.Context) platformpostgres.DBTX { return getDB(ctx, r.db) }
