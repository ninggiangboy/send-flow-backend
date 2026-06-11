package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type MessageReadRepository struct {
	db platformpostgres.DBTX
}

type MessageWriteRepository struct {
	db platformpostgres.DBTX
}

type AttemptReadRepository struct {
	db platformpostgres.DBTX
}

type AttemptWriteRepository struct {
	db platformpostgres.DBTX
}

type OutboxRepository struct {
	db platformpostgres.DBTX
}

func NewMessageReadRepository(db platformpostgres.DBTX) *MessageReadRepository {
	return &MessageReadRepository{db: db}
}

func NewMessageWriteRepository(db platformpostgres.DBTX) *MessageWriteRepository {
	return &MessageWriteRepository{db: db}
}

func NewAttemptReadRepository(db platformpostgres.DBTX) *AttemptReadRepository {
	return &AttemptReadRepository{db: db}
}

func NewAttemptWriteRepository(db platformpostgres.DBTX) *AttemptWriteRepository {
	return &AttemptWriteRepository{db: db}
}

func NewOutboxRepository(db platformpostgres.DBTX) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *MessageReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (w *MessageWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return w.db
}

func (r *AttemptReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (w *AttemptWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return w.db
}

func (r *OutboxRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *MessageReadRepository) FindByID(ctx context.Context, id string) (*domain.NotificationMessage, error) {
	db := r.getDB(ctx)
	var m domain.NotificationMessage
	var workspaceID, recipientUserID, bodyHTML *string
	var lastAttemptAt *time.Time

	err := db.QueryRow(ctx,
		`SELECT id, workspace_id, type, status, recipient_email, recipient_user_id, subject, body_text, body_html, max_attempts, attempt_count, last_attempt_at, created_at, updated_at
		 FROM notification_messages WHERE id = $1`,
		id,
	).Scan(
		&m.ID, &workspaceID, &m.Type, &m.Status, &m.RecipientEmail, &recipientUserID,
		&m.Subject, &m.BodyText, &bodyHTML, &m.MaxAttempts, &m.AttemptCount,
		&lastAttemptAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotificationNotFound
		}
		return nil, err
	}
	m.WorkspaceID = workspaceID
	m.RecipientUserID = recipientUserID
	m.BodyHTML = bodyHTML
	m.LastAttemptAt = lastAttemptAt
	return &m, nil
}

func (r *MessageReadRepository) List(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
	db := r.getDB(ctx)
	args := make([]any, 0)
	where := ""
	argIdx := 1

	if filter.WorkspaceID != nil {
		where += " AND workspace_id = $" + platformpostgres.Itoa(argIdx)
		args = append(args, *filter.WorkspaceID)
		argIdx++
	}
	if filter.RecipientUserID != "" {
		where += " AND recipient_user_id = $" + platformpostgres.Itoa(argIdx)
		args = append(args, filter.RecipientUserID)
		argIdx++
	}
	if filter.Type != "" {
		where += " AND type = $" + platformpostgres.Itoa(argIdx)
		args = append(args, filter.Type)
		argIdx++
	}
	if filter.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.From != nil {
		where += " AND created_at >= $" + platformpostgres.Itoa(argIdx)
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		where += " AND created_at <= $" + platformpostgres.Itoa(argIdx)
		args = append(args, *filter.To)
		argIdx++
	}
	if filter.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM notification_messages WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, filter.Cursor)
		argIdx++
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}

	query := `SELECT id, workspace_id, type, status, recipient_email, recipient_user_id, subject, body_text, body_html, max_attempts, attempt_count, last_attempt_at, created_at, updated_at
		FROM notification_messages WHERE 1=1 ` + where + ` ORDER BY created_at DESC, id DESC LIMIT $` + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)
	argIdx++

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.NotificationMessage
	for rows.Next() {
		var m domain.NotificationMessage
		var workspaceID, recipientUserID, bodyHTML *string
		var lastAttemptAt *time.Time
		if err := rows.Scan(
			&m.ID, &workspaceID, &m.Type, &m.Status, &m.RecipientEmail, &recipientUserID,
			&m.Subject, &m.BodyText, &bodyHTML, &m.MaxAttempts, &m.AttemptCount,
			&lastAttemptAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, "", err
		}
		m.WorkspaceID = workspaceID
		m.RecipientUserID = recipientUserID
		m.BodyHTML = bodyHTML
		m.LastAttemptAt = lastAttemptAt
		results = append(results, m)
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
		results = []domain.NotificationMessage{}
	}
	return results, nextCursor, nil
}

func (r *MessageReadRepository) FindPendingForRetry(ctx context.Context, limit int) ([]domain.NotificationMessage, error) {
	db := r.getDB(ctx)
	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, type, status, recipient_email, recipient_user_id, subject, body_text, body_html, max_attempts, attempt_count, last_attempt_at, created_at, updated_at
		 FROM notification_messages WHERE status = 'retrying' AND attempt_count < max_attempts
		 ORDER BY created_at ASC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.NotificationMessage
	for rows.Next() {
		var m domain.NotificationMessage
		var workspaceID, recipientUserID, bodyHTML *string
		var lastAttemptAt *time.Time
		if err := rows.Scan(
			&m.ID, &workspaceID, &m.Type, &m.Status, &m.RecipientEmail, &recipientUserID,
			&m.Subject, &m.BodyText, &bodyHTML, &m.MaxAttempts, &m.AttemptCount,
			&lastAttemptAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m.WorkspaceID = workspaceID
		m.RecipientUserID = recipientUserID
		m.BodyHTML = bodyHTML
		m.LastAttemptAt = lastAttemptAt
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.NotificationMessage{}
	}
	return results, nil
}

func (w *MessageWriteRepository) Create(ctx context.Context, msg domain.NotificationMessage) error {
	db := w.getDB(ctx)
	_, err := db.Exec(ctx,
		`INSERT INTO notification_messages (id, workspace_id, type, status, recipient_email, recipient_user_id, subject, body_text, body_html, max_attempts, attempt_count, last_attempt_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		msg.ID, msg.WorkspaceID, msg.Type, msg.Status, msg.RecipientEmail,
		msg.RecipientUserID, msg.Subject, msg.BodyText, msg.BodyHTML,
		msg.MaxAttempts, msg.AttemptCount, msg.LastAttemptAt,
		msg.CreatedAt, msg.UpdatedAt,
	)
	return err
}

func (w *MessageWriteRepository) ClaimRetryingMessages(ctx context.Context, limit int) ([]domain.NotificationMessage, error) {
	db := w.getDB(ctx)
	rows, err := db.Query(ctx,
		`UPDATE notification_messages SET status='sending', updated_at=NOW()
		 WHERE id IN (
			 SELECT id FROM notification_messages
			 WHERE status='retrying' AND attempt_count < max_attempts
			 ORDER BY created_at ASC LIMIT $1
			 FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, workspace_id, type, status, recipient_email, recipient_user_id, subject, body_text, body_html, max_attempts, attempt_count, last_attempt_at, created_at, updated_at`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.NotificationMessage
	for rows.Next() {
		var m domain.NotificationMessage
		var workspaceID, recipientUserID, bodyHTML *string
		var lastAttemptAt *time.Time
		if err := rows.Scan(
			&m.ID, &workspaceID, &m.Type, &m.Status, &m.RecipientEmail, &recipientUserID,
			&m.Subject, &m.BodyText, &bodyHTML, &m.MaxAttempts, &m.AttemptCount,
			&lastAttemptAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m.WorkspaceID = workspaceID
		m.RecipientUserID = recipientUserID
		m.BodyHTML = bodyHTML
		m.LastAttemptAt = lastAttemptAt
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.NotificationMessage{}
	}
	return results, nil
}

func (w *MessageWriteRepository) Update(ctx context.Context, msg domain.NotificationMessage) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE notification_messages SET status=$1, attempt_count=$2, last_attempt_at=$3, updated_at=$4
		 WHERE id=$5`,
		msg.Status, msg.AttemptCount, msg.LastAttemptAt, msg.UpdatedAt, msg.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotificationNotFound
	}
	return nil
}

func (r *AttemptReadRepository) FindByMessageID(ctx context.Context, notificationMessageID string) ([]domain.NotificationAttempt, error) {
	db := r.getDB(ctx)
	rows, err := db.Query(ctx,
		`SELECT id, notification_message_id, attempt_number, status, provider, provider_message_id, error_message, attempted_at
		 FROM notification_attempts WHERE notification_message_id = $1
		 ORDER BY attempt_number ASC`,
		notificationMessageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.NotificationAttempt
	for rows.Next() {
		var a domain.NotificationAttempt
		if err := rows.Scan(
			&a.ID, &a.NotificationMessageID, &a.AttemptNumber, &a.Status,
			&a.Provider, &a.ProviderMessageID, &a.ErrorMessage, &a.AttemptedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.NotificationAttempt{}
	}
	return results, nil
}

func (w *AttemptWriteRepository) Create(ctx context.Context, attempt domain.NotificationAttempt) error {
	db := w.getDB(ctx)
	_, err := db.Exec(ctx,
		`INSERT INTO notification_attempts (id, notification_message_id, attempt_number, status, provider, provider_message_id, error_message, attempted_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		attempt.ID, attempt.NotificationMessageID, attempt.AttemptNumber,
		attempt.Status, attempt.Provider, attempt.ProviderMessageID,
		attempt.ErrorMessage, attempt.AttemptedAt,
	)
	return err
}

func (r *OutboxRepository) Save(ctx context.Context, event ports.OutboxEvent) error {
	db := r.getDB(ctx)
	headersJSON, err := json.Marshal(event.Headers)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx,
		`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers, workspace_id, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		event.ID, event.AggregateType, event.AggregateID, event.EventType,
		event.Payload, headersJSON, event.WorkspaceID, event.OccurredAt,
	)
	return err
}
