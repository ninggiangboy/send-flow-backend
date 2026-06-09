package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type ctxKey string

const ctxTxKey ctxKey = "operations_tx"

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) db(ctx context.Context) DBTX {
	tx, ok := ctx.Value(ctxTxKey).(pgx.Tx)
	if ok {
		return tx
	}
	return r.pool
}

func (r *OutboxRepository) GetSummary(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error) {
	args := []any{workspaceID}
	where := "WHERE workspace_id = $1"
	idx := 2

	if filter.EventType != "" {
		where += fmt.Sprintf(" AND event_type = $%d", idx)
		args = append(args, filter.EventType)
		idx++
	}
	if filter.AggregateType != "" {
		where += fmt.Sprintf(" AND aggregate_type = $%d", idx)
		args = append(args, filter.AggregateType)
		idx++
	}
	if filter.AggregateID != "" {
		where += fmt.Sprintf(" AND aggregate_id = $%d", idx)
		args = append(args, filter.AggregateID)
		idx++
	}
	if filter.From != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", idx)
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", idx)
		args = append(args, *filter.To)
		idx++
	}

	var summary domain.OutboxSummary
	var oldestAt *time.Time

	err := r.db(ctx).QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), MIN(created_at)
		FROM outbox_events %s`, where), args...).Scan(&summary.TotalCount, &oldestAt)
	if err != nil {
		return summary, err
	}
	if oldestAt != nil {
		summary.OldestAt = oldestAt
		summary.OldestAgeSec = int64(time.Since(*oldestAt).Seconds())
	}

	eventArgs := make([]any, len(args))
	copy(eventArgs, args)
	eventWhere := where
	rows, err := r.db(ctx).Query(ctx, fmt.Sprintf(`
		SELECT event_type, COUNT(*)
		FROM outbox_events %s
		GROUP BY event_type`, eventWhere), eventArgs...)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	summary.ByEventType = make(map[string]int)
	for rows.Next() {
		var eventType string
		var count int
		if err := rows.Scan(&eventType, &count); err != nil {
			return summary, err
		}
		summary.ByEventType[eventType] = count
	}
	return summary, rows.Err()
}

func (r *OutboxRepository) FindByID(ctx context.Context, workspaceID, outboxID string) (*domain.OutboxRecord, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, workspace_id, aggregate_type, aggregate_id, event_type, payload, headers, occurred_at, created_at
		FROM outbox_events
		WHERE id = $1 AND workspace_id = $2`, outboxID, workspaceID)

	rec, err := scanOutboxRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOutboxRecordNotFound
		}
		return nil, err
	}
	return rec, nil
}

func scanOutboxRecord(row pgx.Row) (*domain.OutboxRecord, error) {
	var rec domain.OutboxRecord
	err := row.Scan(&rec.ID, &rec.WorkspaceID, &rec.AggregateType, &rec.AggregateID,
		&rec.EventType, &rec.Payload, &rec.Headers, &rec.OccurredAt, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

type cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func encodeCursor(createdAt time.Time, id string) string {
	c := cursor{CreatedAt: createdAt, ID: id}
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(data)
}

func decodeCursor(raw string) (cursor, error) {
	data, err := base64.URLEncoding.DecodeString(raw)
	if err != nil {
		return cursor{}, domain.ErrFilterInvalid
	}
	var c cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return cursor{}, domain.ErrFilterInvalid
	}
	return c, nil
}

func (r *OutboxRepository) List(ctx context.Context, workspaceID string, filter domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
	args := []any{workspaceID}
	where := "WHERE workspace_id = $1"
	idx := 2

	if filter.EventType != "" {
		where += fmt.Sprintf(" AND event_type = $%d", idx)
		args = append(args, filter.EventType)
		idx++
	}
	if filter.AggregateType != "" {
		where += fmt.Sprintf(" AND aggregate_type = $%d", idx)
		args = append(args, filter.AggregateType)
		idx++
	}
	if filter.AggregateID != "" {
		where += fmt.Sprintf(" AND aggregate_id = $%d", idx)
		args = append(args, filter.AggregateID)
		idx++
	}
	if filter.From != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", idx)
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", idx)
		args = append(args, *filter.To)
		idx++
	}
	if filter.Cursor != "" {
		c, err := decodeCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		where += fmt.Sprintf(" AND (created_at, id) < ($%d::timestamptz, $%d)", idx, idx+1)
		args = append(args, c.CreatedAt, c.ID)
		idx += 2
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := fmt.Sprintf(`
		SELECT id, workspace_id, aggregate_type, aggregate_id, event_type, payload, headers, occurred_at, created_at
		FROM outbox_events %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, where, idx)
	args = append(args, limit+1)

	rows, err := r.db(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var records []domain.OutboxRecord
	for rows.Next() {
		rec, err := scanOutboxRecord(rows)
		if err != nil {
			return nil, "", err
		}
		records = append(records, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(records) > limit {
		records = records[:limit]
		last := records[len(records)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return records, nextCursor, nil
}

func (r *OutboxRepository) CreateReplayOutboxEvent(ctx context.Context, event domain.OutboxRecord) error {
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers, workspace_id, occurred_at, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9)`,
		event.ID, event.AggregateType, event.AggregateID, event.EventType,
		event.Payload, event.Headers, event.WorkspaceID, event.OccurredAt, event.CreatedAt)
	return err
}

type DeadLetterRepository struct {
	pool *pgxpool.Pool
}

func NewDeadLetterRepository(pool *pgxpool.Pool) *DeadLetterRepository {
	return &DeadLetterRepository{pool: pool}
}

func (r *DeadLetterRepository) db(ctx context.Context) DBTX {
	tx, ok := ctx.Value(ctxTxKey).(pgx.Tx)
	if ok {
		return tx
	}
	return r.pool
}

func (r *DeadLetterRepository) FindByID(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, COALESCE(workspace_id, ''), source, COALESCE(event_id::text, ''), payload, error_message, retryable, failed_at
		FROM dead_letter_records
		WHERE id = $1 AND (workspace_id = $2 OR workspace_id IS NULL)`, id, workspaceID)

	rec, err := scanDeadLetterRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDeadLetterRecordNotFound
		}
		return nil, err
	}
	if rec.WorkspaceID == "" || rec.WorkspaceID != workspaceID {
		return nil, domain.ErrDeadLetterRecordNotFound
	}
	return rec, nil
}

func scanDeadLetterRecord(row pgx.Row) (*domain.DeadLetterRecord, error) {
	var rec domain.DeadLetterRecord
	err := row.Scan(&rec.ID, &rec.WorkspaceID, &rec.Source, &rec.EventID,
		&rec.Payload, &rec.ErrorMessage, &rec.Retryable, &rec.FailedAt)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *DeadLetterRepository) List(ctx context.Context, workspaceID string, filter domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error) {
	args := []any{workspaceID}
	where := "WHERE workspace_id = $1"
	idx := 2

	if filter.Source != "" {
		where += fmt.Sprintf(" AND source = $%d", idx)
		args = append(args, filter.Source)
		idx++
	}
	if filter.Retryable != nil {
		where += fmt.Sprintf(" AND retryable = $%d", idx)
		args = append(args, *filter.Retryable)
		idx++
	}
	if filter.From != nil {
		where += fmt.Sprintf(" AND failed_at >= $%d", idx)
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		where += fmt.Sprintf(" AND failed_at <= $%d", idx)
		args = append(args, *filter.To)
		idx++
	}
	if filter.Cursor != "" {
		c, err := decodeCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		where += fmt.Sprintf(" AND (failed_at, id) < ($%d::timestamptz, $%d)", idx, idx+1)
		args = append(args, c.CreatedAt, c.ID)
		idx += 2
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := fmt.Sprintf(`
		SELECT id, COALESCE(workspace_id, ''), source, COALESCE(event_id::text, ''), payload, error_message, retryable, failed_at
		FROM dead_letter_records %s
		ORDER BY failed_at DESC, id DESC
		LIMIT $%d`, where, idx)
	args = append(args, limit+1)

	rows, err := r.db(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var records []domain.DeadLetterRecord
	for rows.Next() {
		rec, err := scanDeadLetterRecord(rows)
		if err != nil {
			return nil, "", err
		}
		records = append(records, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(records) > limit {
		records = records[:limit]
		last := records[len(records)-1]
		nextCursor = encodeCursor(last.FailedAt, last.ID)
	}
	return records, nextCursor, nil
}

type ReplayJobRepository struct {
	pool *pgxpool.Pool
}

func NewReplayJobRepository(pool *pgxpool.Pool) *ReplayJobRepository {
	return &ReplayJobRepository{pool: pool}
}

func (r *ReplayJobRepository) db(ctx context.Context) DBTX {
	tx, ok := ctx.Value(ctxTxKey).(pgx.Tx)
	if ok {
		return tx
	}
	return r.pool
}

func (r *ReplayJobRepository) Create(ctx context.Context, job domain.ReplayJob) error {
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO replay_jobs (id, workspace_id, target_type, target_id, source, status, requested_by_user_id, reason, filter_json, result_json, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11, $12, $13)`,
		job.ID, job.WorkspaceID, string(job.TargetType), job.TargetID, job.Source,
		string(job.Status), job.RequestedByUserID, job.Reason, job.Filter, job.Result,
		job.ErrorMessage, job.CreatedAt, job.UpdatedAt)
	return err
}

func (r *ReplayJobRepository) FindByID(ctx context.Context, workspaceID, jobID string) (*domain.ReplayJob, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, workspace_id, target_type, target_id, source, status, COALESCE(requested_by_user_id, ''), reason, filter_json, result_json, error_message, created_at, started_at, completed_at, updated_at
		FROM replay_jobs
		WHERE id = $1 AND workspace_id = $2`, jobID, workspaceID)

	return scanReplayJob(row)
}

func scanReplayJob(row pgx.Row) (*domain.ReplayJob, error) {
	var job domain.ReplayJob
	err := row.Scan(&job.ID, &job.WorkspaceID, &job.TargetType, &job.TargetID,
		&job.Source, &job.Status, &job.RequestedByUserID, &job.Reason,
		&job.Filter, &job.Result, &job.ErrorMessage,
		&job.CreatedAt, &job.StartedAt, &job.CompletedAt, &job.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrReplayJobNotFound
		}
		return nil, err
	}
	return &job, nil
}

func (r *ReplayJobRepository) List(ctx context.Context, workspaceID string, filter domain.ReplayJobFilter) ([]domain.ReplayJob, string, error) {
	args := []any{workspaceID}
	where := "WHERE workspace_id = $1"
	idx := 2

	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, filter.Status)
		idx++
	}
	if filter.Cursor != "" {
		c, err := decodeCursor(filter.Cursor)
		if err != nil {
			return nil, "", err
		}
		where += fmt.Sprintf(" AND (created_at, id) < ($%d::timestamptz, $%d)", idx, idx+1)
		args = append(args, c.CreatedAt, c.ID)
		idx += 2
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := fmt.Sprintf(`
		SELECT id, workspace_id, target_type, target_id, source, status, COALESCE(requested_by_user_id, ''), reason, filter_json, result_json, error_message, created_at, started_at, completed_at, updated_at
		FROM replay_jobs %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, where, idx)
	args = append(args, limit+1)

	rows, err := r.db(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var jobs []domain.ReplayJob
	for rows.Next() {
		job, err := scanReplayJob(rows)
		if err != nil {
			return nil, "", err
		}
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(jobs) > limit {
		jobs = jobs[:limit]
		last := jobs[len(jobs)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return jobs, nextCursor, nil
}

func (r *ReplayJobRepository) MarkRunning(ctx context.Context, workspaceID, jobID string, startedAt time.Time) error {
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE replay_jobs SET status = 'running', started_at = $3, updated_at = $3
		WHERE id = $1 AND workspace_id = $2 AND status = 'queued'`,
		jobID, workspaceID, startedAt)
	return err
}

func (r *ReplayJobRepository) MarkCompleted(ctx context.Context, workspaceID, jobID string, result map[string]any, completedAt time.Time) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = r.db(ctx).Exec(ctx, `
		UPDATE replay_jobs SET status = 'completed', result_json = $3::jsonb, completed_at = $4, updated_at = $4
		WHERE id = $1 AND workspace_id = $2`,
		jobID, workspaceID, resultJSON, completedAt)
	return err
}

func (r *ReplayJobRepository) MarkFailed(ctx context.Context, workspaceID, jobID, errorMessage string, completedAt time.Time) error {
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE replay_jobs SET status = 'failed', error_message = $3, completed_at = $4, updated_at = $4
		WHERE id = $1 AND workspace_id = $2`,
		jobID, workspaceID, errorMessage, completedAt)
	return err
}

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTransactionManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

func (tm *TxManager) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(context.WithValue(ctx, ctxTxKey, tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
