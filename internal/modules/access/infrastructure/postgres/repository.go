package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

type apiKeyRow struct {
	ID          string
	WorkspaceID string
	Name        string
	KeyPrefix   string
	SecretHash  string
	Scopes      string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastUsedAt  *time.Time
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
}

type APIKeyRepository struct {
	readDB  DBTX
	writeDB DBTX
}

func NewAPIKeyRepository(readPool, writePool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{readDB: readPool, writeDB: writePool}
}

func (r *APIKeyRepository) getReadDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.readDB
}

func (r *APIKeyRepository) getWriteDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.writeDB
}

func (r *APIKeyRepository) ListByWorkspace(ctx context.Context, query ports.APIKeyListQuery) ([]domain.APIKey, string, error) {
	db := r.getReadDB(ctx)

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	var rows pgx.Rows
	var err error
	if query.Cursor != "" {
		rows, err = db.Query(ctx, `SELECT id,workspace_id,name,key_prefix,secret_hash,scopes::text,status,created_at,updated_at,last_used_at,expires_at,revoked_at FROM api_keys WHERE workspace_id=$1 AND (created_at,id) < ($2,$3) ORDER BY created_at DESC, id DESC LIMIT $4`,
			query.WorkspaceID, query.Cursor, query.Cursor, limit)
	} else if query.Status != "" {
		rows, err = db.Query(ctx, `SELECT id,workspace_id,name,key_prefix,secret_hash,scopes::text,status,created_at,updated_at,last_used_at,expires_at,revoked_at FROM api_keys WHERE workspace_id=$1 AND status=$2 ORDER BY created_at DESC, id DESC LIMIT $3`,
			query.WorkspaceID, query.Status, limit)
	} else {
		rows, err = db.Query(ctx, `SELECT id,workspace_id,name,key_prefix,secret_hash,scopes::text,status,created_at,updated_at,last_used_at,expires_at,revoked_at FROM api_keys WHERE workspace_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2`,
			query.WorkspaceID, limit)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []domain.APIKey
	for rows.Next() {
		var row apiKeyRow
		if err := rows.Scan(&row.ID, &row.WorkspaceID, &row.Name, &row.KeyPrefix, &row.SecretHash, &row.Scopes, &row.Status, &row.CreatedAt, &row.UpdatedAt, &row.LastUsedAt, &row.ExpiresAt, &row.RevokedAt); err != nil {
			return nil, "", err
		}
		key, err := rowToDomain(row)
		if err != nil {
			return nil, "", err
		}
		out = append(out, *key)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var cursor string
	if len(out) > 0 {
		last := out[len(out)-1]
		cursor = last.CreatedAt.Format(time.RFC3339Nano) + "," + last.ID
	}
	return out, cursor, nil
}

func (r *APIKeyRepository) FindByID(ctx context.Context, workspaceID, keyID string) (*domain.APIKey, error) {
	return r.findOne(ctx, r.getReadDB(ctx), `SELECT id,workspace_id,name,key_prefix,secret_hash,scopes::text,status,created_at,updated_at,last_used_at,expires_at,revoked_at FROM api_keys WHERE workspace_id=$1 AND id=$2`, workspaceID, keyID)
}

func (r *APIKeyRepository) FindByPrefix(ctx context.Context, keyPrefix string) (*domain.APIKey, error) {
	return r.findOne(ctx, r.getReadDB(ctx), `SELECT id,workspace_id,name,key_prefix,secret_hash,scopes::text,status,created_at,updated_at,last_used_at,expires_at,revoked_at FROM api_keys WHERE key_prefix=$1`, keyPrefix)
}

func (r *APIKeyRepository) Create(ctx context.Context, key domain.APIKey) error {
	scopesJSON, err := json.Marshal(key.Scopes)
	if err != nil {
		return err
	}
	db := r.getWriteDB(ctx)
	_, err = db.Exec(ctx, `INSERT INTO api_keys (id,workspace_id,name,key_prefix,secret_hash,scopes,status,created_at,updated_at,last_used_at,expires_at,revoked_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		key.ID, key.WorkspaceID, key.Name, key.KeyPrefix, key.SecretHash, scopesJSON, string(key.Status), key.CreatedAt, key.UpdatedAt, key.LastUsedAt, key.ExpiresAt, key.RevokedAt)
	return err
}

func (r *APIKeyRepository) Update(ctx context.Context, key domain.APIKey) error {
	scopesJSON, err := json.Marshal(key.Scopes)
	if err != nil {
		return err
	}
	db := r.getWriteDB(ctx)
	_, err = db.Exec(ctx, `UPDATE api_keys SET name=$3, key_prefix=$4, secret_hash=$5, scopes=$6, status=$7, updated_at=$8, expires_at=$9, revoked_at=$10 WHERE workspace_id=$1 AND id=$2`,
		key.WorkspaceID, key.ID, key.Name, key.KeyPrefix, key.SecretHash, scopesJSON, string(key.Status), key.UpdatedAt, key.ExpiresAt, key.RevokedAt)
	return err
}

func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, workspaceID, keyID string, usedAt time.Time) error {
	db := r.getWriteDB(ctx)
	_, err := db.Exec(ctx, `UPDATE api_keys SET last_used_at=$3, updated_at=$3 WHERE workspace_id=$1 AND id=$2 AND status='active'`, workspaceID, keyID, usedAt)
	return err
}

func (r *APIKeyRepository) findOne(ctx context.Context, db DBTX, sql string, args ...any) (*domain.APIKey, error) {
	var row apiKeyRow
	err := db.QueryRow(ctx, sql, args...).Scan(&row.ID, &row.WorkspaceID, &row.Name, &row.KeyPrefix, &row.SecretHash, &row.Scopes, &row.Status, &row.CreatedAt, &row.UpdatedAt, &row.LastUsedAt, &row.ExpiresAt, &row.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToDomain(row)
}

func rowToDomain(row apiKeyRow) (*domain.APIKey, error) {
	var scopes []string
	if row.Scopes != "" {
		if err := json.Unmarshal([]byte(row.Scopes), &scopes); err != nil {
			return nil, err
		}
	}
	return &domain.APIKey{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		KeyPrefix:   row.KeyPrefix,
		SecretHash:  row.SecretHash,
		Scopes:      scopes,
		Status:      domain.APIKeyStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		LastUsedAt:  row.LastUsedAt,
		ExpiresAt:   row.ExpiresAt,
		RevokedAt:   row.RevokedAt,
	}, nil
}
