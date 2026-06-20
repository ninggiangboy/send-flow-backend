package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type WriteRepository struct {
	db platformpostgres.DBTX
}

func NewWriteRepository(db platformpostgres.DBTX) *WriteRepository {
	return &WriteRepository{db: db}
}

func (r *WriteRepository) Append(ctx context.Context, entry auditdomain.AuditEntry) error {
	payload, err := json.Marshal(entry.PayloadSummary)
	if err != nil {
		return fmt.Errorf("marshal payload summary: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO audit_entries (id, workspace_id, actor_user_id, action_type, target_type, target_id, payload_summary, request_id, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, entry.ID, entry.WorkspaceID, platformpostgres.Nullable(entry.ActorUserID), entry.ActionType, entry.TargetType, entry.TargetID, payload, entry.RequestID, entry.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}

func (r *WriteRepository) List(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}

	query := `SELECT id, workspace_id, actor_user_id, action_type, target_type, target_id, payload_summary, request_id, occurred_at
		FROM audit_entries WHERE workspace_id = $1`
	args := []any{filter.WorkspaceID}
	argIdx := 2

	if filter.ActorUserID != "" {
		query += fmt.Sprintf(" AND actor_user_id = $%d", argIdx)
		args = append(args, filter.ActorUserID)
		argIdx++
	}
	if filter.ActionType != "" {
		query += fmt.Sprintf(" AND action_type = $%d", argIdx)
		args = append(args, filter.ActionType)
		argIdx++
	}
	if filter.TargetType != "" {
		query += fmt.Sprintf(" AND target_type = $%d", argIdx)
		args = append(args, filter.TargetType)
		argIdx++
	}
	if filter.TargetID != "" {
		query += fmt.Sprintf(" AND target_id = $%d", argIdx)
		args = append(args, filter.TargetID)
		argIdx++
	}
	if filter.From != nil {
		query += fmt.Sprintf(" AND occurred_at >= $%d", argIdx)
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		query += fmt.Sprintf(" AND occurred_at <= $%d", argIdx)
		args = append(args, *filter.To)
		argIdx++
	}

	if filter.Cursor != "" {
		cursorParts := strings.SplitN(filter.Cursor, "_", 2)
		if len(cursorParts) == 2 {
			query += fmt.Sprintf(" AND (occurred_at, id) < ($%d::timestamptz, $%d)", argIdx, argIdx+1)
			args = append(args, cursorParts[0], cursorParts[1])
			argIdx += 2
		}
	}

	query += " ORDER BY occurred_at DESC, id DESC"

	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query audit entries: %w", err)
	}
	defer rows.Close()

	var entries []auditdomain.AuditEntry
	for rows.Next() {
		var entry auditdomain.AuditEntry
		var payload []byte
		var actorUserID *string
		if err := rows.Scan(&entry.ID, &entry.WorkspaceID, &actorUserID, &entry.ActionType, &entry.TargetType, &entry.TargetID, &payload, &entry.RequestID, &entry.OccurredAt); err != nil {
			return nil, "", fmt.Errorf("scan audit entry: %w", err)
		}
		if actorUserID != nil {
			entry.ActorUserID = *actorUserID
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &entry.PayloadSummary); err != nil {
				entry.PayloadSummary = map[string]any{"_unparseable": string(payload)}
			}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("rows iteration: %w", err)
	}

	var nextCursor string
	if len(entries) > limit {
		entries = entries[:limit]
		if len(entries) > 0 {
			last := entries[len(entries)-1]
			nextCursor = fmt.Sprintf("%s_%s", last.OccurredAt.Format(time.RFC3339Nano), last.ID)
		}
	}

	return entries, nextCursor, nil
}

type ReadRepository struct {
	db platformpostgres.DBTX
}

func NewReadRepository(db platformpostgres.DBTX) *ReadRepository {
	return &ReadRepository{db: db}
}

func (r *ReadRepository) List(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}

	query := `SELECT id, workspace_id, actor_user_id, action_type, target_type, target_id, payload_summary, request_id, occurred_at
		FROM audit_entries WHERE workspace_id = $1`
	args := []any{filter.WorkspaceID}
	argIdx := 2

	if filter.ActorUserID != "" {
		query += fmt.Sprintf(" AND actor_user_id = $%d", argIdx)
		args = append(args, filter.ActorUserID)
		argIdx++
	}
	if filter.ActionType != "" {
		query += fmt.Sprintf(" AND action_type = $%d", argIdx)
		args = append(args, filter.ActionType)
		argIdx++
	}
	if filter.TargetType != "" {
		query += fmt.Sprintf(" AND target_type = $%d", argIdx)
		args = append(args, filter.TargetType)
		argIdx++
	}
	if filter.TargetID != "" {
		query += fmt.Sprintf(" AND target_id = $%d", argIdx)
		args = append(args, filter.TargetID)
		argIdx++
	}
	if filter.From != nil {
		query += fmt.Sprintf(" AND occurred_at >= $%d", argIdx)
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		query += fmt.Sprintf(" AND occurred_at <= $%d", argIdx)
		args = append(args, *filter.To)
		argIdx++
	}

	if filter.Cursor != "" {
		cursorParts := strings.SplitN(filter.Cursor, "_", 2)
		if len(cursorParts) == 2 {
			query += fmt.Sprintf(" AND (occurred_at, id) < ($%d::timestamptz, $%d)", argIdx, argIdx+1)
			args = append(args, cursorParts[0], cursorParts[1])
			argIdx += 2
		}
	}

	query += " ORDER BY occurred_at DESC, id DESC"

	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query audit entries: %w", err)
	}
	defer rows.Close()

	var entries []auditdomain.AuditEntry
	for rows.Next() {
		var entry auditdomain.AuditEntry
		var payload []byte
		var actorUserID *string
		if err := rows.Scan(&entry.ID, &entry.WorkspaceID, &actorUserID, &entry.ActionType, &entry.TargetType, &entry.TargetID, &payload, &entry.RequestID, &entry.OccurredAt); err != nil {
			return nil, "", fmt.Errorf("scan audit entry: %w", err)
		}
		if actorUserID != nil {
			entry.ActorUserID = *actorUserID
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &entry.PayloadSummary); err != nil {
				entry.PayloadSummary = map[string]any{"_unparseable": string(payload)}
			}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("rows iteration: %w", err)
	}

	var nextCursor string
	if len(entries) > limit {
		entries = entries[:limit]
		if len(entries) > 0 {
			last := entries[len(entries)-1]
			nextCursor = fmt.Sprintf("%s_%s", last.OccurredAt.Format(time.RFC3339Nano), last.ID)
		}
	}

	return entries, nextCursor, nil
}

// Combined repository — satisfies the merged EntryWriteRepository interface.

type EntryRepository struct {
	*ReadRepository
	*WriteRepository
}

func NewEntryRepository(read *ReadRepository, write *WriteRepository) *EntryRepository {
	return &EntryRepository{read, write}
}
