package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type ReadRepository struct {
	db DBTX
}

type WriteRepository struct {
	db DBTX
}

func NewReadRepository(db DBTX) *ReadRepository {
	return &ReadRepository{db: db}
}

func NewWriteRepository(db DBTX) *WriteRepository {
	return &WriteRepository{db: db}
}

func (r *ReadRepository) FindByID(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error) {
	var e domain.SuppressionEntry
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, email, email_normalized, scope, reason, status, COALESCE(note, ''), created_at, updated_at, removed_at FROM suppression_entries WHERE id = $1 AND workspace_id = $2`,
		entryID, workspaceID,
	).Scan(&e.ID, &e.WorkspaceID, &e.Email, &e.EmailNormalized, &e.Scope, &e.Reason, &e.Status, &e.Note, &e.CreatedAt, &e.UpdatedAt, &e.RemovedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrEntryNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (r *ReadRepository) List(ctx context.Context, query ports.SuppressionListQuery) ([]domain.SuppressionEntry, string, error) {
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if query.Email != "" {
		where += " AND email_normalized = $" + itoa(argIdx)
		args = append(args, query.Email)
		argIdx++
	}
	if query.Scope != "" {
		where += " AND scope = $" + itoa(argIdx)
		args = append(args, query.Scope)
		argIdx++
	}
	if query.Reason != "" {
		where += " AND reason = $" + itoa(argIdx)
		args = append(args, query.Reason)
		argIdx++
	}
	if query.From != nil {
		where += " AND created_at >= $" + itoa(argIdx)
		args = append(args, *query.From)
		argIdx++
	}
	if query.To != nil {
		where += " AND created_at <= $" + itoa(argIdx)
		args = append(args, *query.To)
		argIdx++
	}
	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM suppression_entries WHERE id = $" + itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, email, email_normalized, scope, reason, status, COALESCE(note, ''), created_at, updated_at, removed_at FROM suppression_entries `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.SuppressionEntry
	for rows.Next() {
		var e domain.SuppressionEntry
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.Email, &e.EmailNormalized, &e.Scope, &e.Reason, &e.Status, &e.Note, &e.CreatedAt, &e.UpdatedAt, &e.RemovedAt); err != nil {
			return nil, "", err
		}
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit-1].ID
		results = results[:limit]
	}
	if results == nil {
		results = []domain.SuppressionEntry{}
	}
	return results, nextCursor, nil
}

func (w *WriteRepository) Create(ctx context.Context, e domain.SuppressionEntry) error {
	_, err := w.db.Exec(ctx,
		`INSERT INTO suppression_entries (id, workspace_id, email, email_normalized, scope, reason, status, note, created_at, updated_at, removed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.ID, e.WorkspaceID, e.Email, e.EmailNormalized, string(e.Scope), string(e.Reason), string(e.Status), nullable(e.Note), e.CreatedAt, e.UpdatedAt, e.RemovedAt,
	)
	return err
}

func (r *ReadRepository) FindActiveByEmail(ctx context.Context, query ports.SuppressionCheckQuery) (*domain.SuppressionEntry, error) {
	args := []any{query.WorkspaceID, query.EmailNormalized, domain.SuppressionStatusActive}
	where := "WHERE workspace_id = $1 AND email_normalized = $2 AND status = $3"
	argIdx := 4

	if len(query.Scopes) > 0 {
		placeholders := make([]string, len(query.Scopes))
		for i, s := range query.Scopes {
			placeholders[i] = "$" + itoa(argIdx)
			args = append(args, s)
			argIdx++
		}
		where += " AND scope IN (" + strings.Join(placeholders, ",") + ")"
	}
	if len(query.Reasons) > 0 {
		placeholders := make([]string, len(query.Reasons))
		for i, r := range query.Reasons {
			placeholders[i] = "$" + itoa(argIdx)
			args = append(args, r)
			argIdx++
		}
		where += " AND reason IN (" + strings.Join(placeholders, ",") + ")"
	}

	where += " ORDER BY reason = 'complaint' DESC, reason = 'bounce' DESC, reason = 'unsubscribe' DESC, reason = 'manual_block' DESC, created_at DESC LIMIT 1"

	var e domain.SuppressionEntry
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, email, email_normalized, scope, reason, status, COALESCE(note, ''), created_at, updated_at, removed_at FROM suppression_entries `+where, args...).Scan(
		&e.ID, &e.WorkspaceID, &e.Email, &e.EmailNormalized, &e.Scope, &e.Reason, &e.Status, &e.Note, &e.CreatedAt, &e.UpdatedAt, &e.RemovedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (w *WriteRepository) Remove(ctx context.Context, workspaceID, entryID string, removedAt time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE suppression_entries SET status='removed', removed_at=$1, updated_at=$1 WHERE id=$2 AND workspace_id=$3`,
		removedAt, entryID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEntryNotFound
	}
	return nil
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [12]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
