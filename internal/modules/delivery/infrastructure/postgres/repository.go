package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type scannable interface {
	Scan(dest ...any) error
}

func scanMessageRow(row scannable) (*domain.Message, error) {
	var msg domain.Message
	var snapshotJSON []byte
	var scheduledAt, queuedAt, processingStartedAt, acceptedAt, deliveredAt, bouncedAt, complainedAt, failedAt *time.Time

	err := row.Scan(
		&msg.ID, &msg.WorkspaceID, &msg.CampaignID, &msg.CampaignCandidateID, &msg.TransactionalRequestID,
		&msg.ContactID, &msg.RecipientEmailNormalized, &snapshotJSON,
		&msg.TemplateID, &msg.TemplateVersionID, &msg.SenderDomainID,
		&msg.MessageType, &msg.SourceType, &msg.Status,
		&scheduledAt, &queuedAt, &processingStartedAt,
		&acceptedAt, &deliveredAt, &bouncedAt, &complainedAt, &failedAt,
		&msg.LastErrorClass, &msg.LastErrorMessage, &msg.Provider, &msg.ProviderMessageID,
		&msg.CreatedAt, &msg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMessageNotFound
		}
		return nil, err
	}

	if snapshotJSON != nil {
		if err := json.Unmarshal(snapshotJSON, &msg.RecipientSnapshot); err != nil {
			return nil, err
		}
	}
	msg.ScheduledAt = scheduledAt
	msg.QueuedAt = queuedAt
	msg.ProcessingStartedAt = processingStartedAt
	msg.AcceptedAt = acceptedAt
	msg.DeliveredAt = deliveredAt
	msg.BouncedAt = bouncedAt
	msg.ComplainedAt = complainedAt
	msg.FailedAt = failedAt

	return &msg, nil
}

func scanMessageRows(rows pgx.Rows) ([]domain.Message, error) {
	var results []domain.Message
	for rows.Next() {
		msg, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.Message{}
	}
	return results, nil
}

type MessageReadRepository struct {
	db platformpostgres.DBTX
}

type MessageWriteRepository struct {
	db platformpostgres.DBTX
}

func NewMessageReadRepository(db platformpostgres.DBTX) *MessageReadRepository {
	return &MessageReadRepository{db: db}
}

func NewMessageWriteRepository(db platformpostgres.DBTX) *MessageWriteRepository {
	return &MessageWriteRepository{db: db}
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

func (r *MessageReadRepository) FindByID(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
	db := r.getDB(ctx)
	return scanMessageRow(db.QueryRow(ctx,
		`SELECT id, workspace_id, COALESCE(campaign_id, ''), COALESCE(campaign_candidate_id, ''), COALESCE(transactional_request_id, ''),
		        COALESCE(contact_id, ''), recipient_email_normalized, recipient_snapshot,
		        COALESCE(template_id, ''), COALESCE(template_version_id, ''), COALESCE(sender_domain_id, ''),
		        message_type, source_type, status,
		        scheduled_at, queued_at, processing_started_at,
		        accepted_at, delivered_at, bounced_at, complained_at, failed_at,
		        last_error_class, last_error_message, provider, provider_message_id,
		        created_at, updated_at
		 FROM messages WHERE id = $1 AND workspace_id = $2`,
		messageID, workspaceID,
	))
}

func (r *MessageReadRepository) FindByIDForUpdate(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
	db := r.getDB(ctx)
	return scanMessageRow(db.QueryRow(ctx,
		`SELECT id, workspace_id, COALESCE(campaign_id, ''), COALESCE(campaign_candidate_id, ''), COALESCE(transactional_request_id, ''),
		        COALESCE(contact_id, ''), recipient_email_normalized, recipient_snapshot,
		        COALESCE(template_id, ''), COALESCE(template_version_id, ''), COALESCE(sender_domain_id, ''),
		        message_type, source_type, status,
		        scheduled_at, queued_at, processing_started_at,
		        accepted_at, delivered_at, bounced_at, complained_at, failed_at,
		        last_error_class, last_error_message, provider, provider_message_id,
		        created_at, updated_at
		 FROM messages WHERE id = $1 AND workspace_id = $2 FOR UPDATE`,
		messageID, workspaceID,
	))
}

func (r *MessageReadRepository) FindByTransactionalRequestID(ctx context.Context, workspaceID, transactionalRequestID string) (*domain.Message, error) {
	db := r.getDB(ctx)
	return scanMessageRow(db.QueryRow(ctx,
		`SELECT id, workspace_id, COALESCE(campaign_id, ''), COALESCE(campaign_candidate_id, ''), COALESCE(transactional_request_id, ''),
		        COALESCE(contact_id, ''), recipient_email_normalized, recipient_snapshot,
		        COALESCE(template_id, ''), COALESCE(template_version_id, ''), COALESCE(sender_domain_id, ''),
		        message_type, source_type, status,
		        scheduled_at, queued_at, processing_started_at,
		        accepted_at, delivered_at, bounced_at, complained_at, failed_at,
		        last_error_class, last_error_message, provider, provider_message_id,
		        created_at, updated_at
		 FROM messages WHERE workspace_id = $1 AND transactional_request_id = $2`,
		workspaceID, transactionalRequestID,
	))
}

func (r *MessageReadRepository) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*domain.Message, error) {
	db := r.getDB(ctx)
	return scanMessageRow(db.QueryRow(ctx,
		`SELECT id, workspace_id, COALESCE(campaign_id, ''), COALESCE(campaign_candidate_id, ''), COALESCE(transactional_request_id, ''),
		        COALESCE(contact_id, ''), recipient_email_normalized, recipient_snapshot,
		        COALESCE(template_id, ''), COALESCE(template_version_id, ''), COALESCE(sender_domain_id, ''),
		        message_type, source_type, status,
		        scheduled_at, queued_at, processing_started_at,
		        accepted_at, delivered_at, bounced_at, complained_at, failed_at,
		        last_error_class, last_error_message, provider, provider_message_id,
		        created_at, updated_at
		 FROM messages WHERE provider = $1 AND provider_message_id = $2`,
		provider, providerMessageID,
	))
}

func (r *MessageReadRepository) List(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error) {
	db := r.getDB(ctx)
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if query.CampaignID != "" {
		where += " AND campaign_id = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.CampaignID)
		argIdx++
	}
	if query.TransactionalRequestID != "" {
		where += " AND transactional_request_id = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.TransactionalRequestID)
		argIdx++
	}
	if query.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.Status)
		argIdx++
	}
	if query.RecipientEmailNormalized != "" {
		where += " AND recipient_email_normalized = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.RecipientEmailNormalized)
		argIdx++
	}
	if query.ProviderMessageID != "" {
		where += " AND provider_message_id = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.ProviderMessageID)
		argIdx++
	}
	if query.From != nil {
		where += " AND created_at >= $" + platformpostgres.Itoa(argIdx)
		args = append(args, *query.From)
		argIdx++
	}
	if query.To != nil {
		where += " AND created_at <= $" + platformpostgres.Itoa(argIdx)
		args = append(args, *query.To)
		argIdx++
	}
	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM messages WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	if limit > 100 {
		limit = 100
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, COALESCE(campaign_id, ''), COALESCE(campaign_candidate_id, ''), COALESCE(transactional_request_id, ''),
		        COALESCE(contact_id, ''), recipient_email_normalized, recipient_snapshot,
		        COALESCE(template_id, ''), COALESCE(template_version_id, ''), COALESCE(sender_domain_id, ''),
		        message_type, source_type, status,
		        scheduled_at, queued_at, processing_started_at,
		        accepted_at, delivered_at, bounced_at, complained_at, failed_at,
		        last_error_class, last_error_message, provider, provider_message_id,
		        created_at, updated_at
		 FROM messages `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	results, err := scanMessageRows(rows)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit-1].ID
		results = results[:limit]
	}
	if results == nil {
		results = []domain.Message{}
	}

	return results, nextCursor, nil
}

func (r *MessageReadRepository) ListDueQueued(ctx context.Context, query ports.DueMessageQuery) ([]domain.Message, error) {
	db := r.getDB(ctx)
	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, COALESCE(campaign_id, ''), COALESCE(campaign_candidate_id, ''), COALESCE(transactional_request_id, ''),
		        COALESCE(contact_id, ''), recipient_email_normalized, recipient_snapshot,
		        COALESCE(template_id, ''), COALESCE(template_version_id, ''), COALESCE(sender_domain_id, ''),
		        message_type, source_type, status,
		        scheduled_at, queued_at, processing_started_at,
		        accepted_at, delivered_at, bounced_at, complained_at, failed_at,
		        last_error_class, last_error_message, provider, provider_message_id,
		        created_at, updated_at
		 FROM messages
		 WHERE workspace_id = $1 AND status = 'queued' AND message_type = $2
		   AND (scheduled_at IS NULL OR scheduled_at <= $3)
		 ORDER BY scheduled_at ASC, created_at ASC LIMIT $4`,
		query.WorkspaceID, query.MessageType, query.Now, query.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessageRows(rows)
}

func (r *MessageReadRepository) ListDistinctWorkspacesWithDue(ctx context.Context, messageType string, now time.Time) ([]string, error) {
	db := r.getDB(ctx)
	rows, err := db.Query(ctx,
		`SELECT DISTINCT workspace_id FROM messages
		 WHERE status = 'queued' AND message_type = $1
		   AND (scheduled_at IS NULL OR scheduled_at <= $2)`,
		messageType, now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []string
	for rows.Next() {
		var ws string
		if err := rows.Scan(&ws); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workspaces, nil
}

func (r *MessageReadRepository) CountByCampaign(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	db := r.getDB(ctx)
	var count int64
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE workspace_id = $1 AND campaign_id = $2`,
		workspaceID, campaignID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (w *MessageWriteRepository) CreateMany(ctx context.Context, messages []domain.Message) ([]string, error) {
	db := w.getDB(ctx)
	if len(messages) == 0 {
		return nil, nil
	}

	values := make([]string, 0, len(messages))
	args := make([]any, 0, len(messages)*28)
	idx := 1

	for _, msg := range messages {
		snapshotJSON, err := json.Marshal(msg.RecipientSnapshot)
		if err != nil {
			return nil, err
		}
		values = append(values, "($"+
			platformpostgres.Itoa(idx)+",$"+platformpostgres.Itoa(idx+1)+",$"+platformpostgres.Itoa(idx+2)+",$"+platformpostgres.Itoa(idx+3)+",$"+platformpostgres.Itoa(idx+4)+",$"+platformpostgres.Itoa(idx+5)+",$"+platformpostgres.Itoa(idx+6)+",$"+platformpostgres.Itoa(idx+7)+",$"+platformpostgres.Itoa(idx+8)+",$"+platformpostgres.Itoa(idx+9)+",$"+platformpostgres.Itoa(idx+10)+",$"+platformpostgres.Itoa(idx+11)+",$"+platformpostgres.Itoa(idx+12)+",$"+platformpostgres.Itoa(idx+13)+",$"+platformpostgres.Itoa(idx+14)+",$"+platformpostgres.Itoa(idx+15)+",$"+platformpostgres.Itoa(idx+16)+",$"+platformpostgres.Itoa(idx+17)+",$"+platformpostgres.Itoa(idx+18)+",$"+platformpostgres.Itoa(idx+19)+",$"+platformpostgres.Itoa(idx+20)+",$"+platformpostgres.Itoa(idx+21)+",$"+platformpostgres.Itoa(idx+22)+",$"+platformpostgres.Itoa(idx+23)+",$"+platformpostgres.Itoa(idx+24)+",$"+platformpostgres.Itoa(idx+25)+",$"+platformpostgres.Itoa(idx+26)+",$"+platformpostgres.Itoa(idx+27)+",$"+platformpostgres.Itoa(idx+28)+")")
		args = append(args, msg.ID, msg.WorkspaceID,
			platformpostgres.Nullable(msg.CampaignID), platformpostgres.Nullable(msg.CampaignCandidateID), platformpostgres.Nullable(msg.TransactionalRequestID),
			platformpostgres.Nullable(msg.ContactID), msg.RecipientEmailNormalized, snapshotJSON,
			platformpostgres.Nullable(msg.TemplateID), platformpostgres.Nullable(msg.TemplateVersionID), platformpostgres.Nullable(msg.SenderDomainID),
			msg.MessageType, msg.SourceType, msg.Status,
			msg.ScheduledAt, msg.QueuedAt, msg.ProcessingStartedAt,
			msg.AcceptedAt, msg.DeliveredAt, msg.BouncedAt, msg.ComplainedAt, msg.FailedAt,
			msg.LastErrorClass, msg.LastErrorMessage, msg.Provider, msg.ProviderMessageID,
			msg.CreatedAt, msg.UpdatedAt)
		idx += 28
	}

	rows, err := db.Query(ctx,
		`INSERT INTO messages (id, workspace_id, campaign_id, campaign_candidate_id, transactional_request_id,
		                       contact_id, recipient_email_normalized, recipient_snapshot,
		                       template_id, template_version_id, sender_domain_id,
		                       message_type, source_type, status,
		                       scheduled_at, queued_at, processing_started_at,
		                       accepted_at, delivered_at, bounced_at, complained_at, failed_at,
		                       last_error_class, last_error_message, provider, provider_message_id,
		                       created_at, updated_at)
		 VALUES `+strings.Join(values, ",")+` ON CONFLICT DO NOTHING RETURNING id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var insertedIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		insertedIDs = append(insertedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return insertedIDs, nil
}

func (w *MessageWriteRepository) Update(ctx context.Context, message domain.Message) error {
	db := w.getDB(ctx)
	snapshotJSON, err := json.Marshal(message.RecipientSnapshot)
	if err != nil {
		return err
	}

	tag, err := db.Exec(ctx,
		`UPDATE messages SET
		        campaign_id=$1, campaign_candidate_id=$2, transactional_request_id=$3,
		        contact_id=$4, recipient_email_normalized=$5, recipient_snapshot=$6,
		        template_id=$7, template_version_id=$8, sender_domain_id=$9,
		        message_type=$10, source_type=$11, status=$12,
		        scheduled_at=$13, queued_at=$14, processing_started_at=$15,
		        accepted_at=$16, delivered_at=$17, bounced_at=$18, complained_at=$19, failed_at=$20,
		        last_error_class=$21, last_error_message=$22, provider=$23, provider_message_id=$24,
		        updated_at=$25
		 WHERE id=$26 AND workspace_id=$27`,
		platformpostgres.Nullable(message.CampaignID), platformpostgres.Nullable(message.CampaignCandidateID), platformpostgres.Nullable(message.TransactionalRequestID),
		platformpostgres.Nullable(message.ContactID), message.RecipientEmailNormalized, snapshotJSON,
		platformpostgres.Nullable(message.TemplateID), platformpostgres.Nullable(message.TemplateVersionID), platformpostgres.Nullable(message.SenderDomainID),
		message.MessageType, message.SourceType, message.Status,
		message.ScheduledAt, message.QueuedAt, message.ProcessingStartedAt,
		message.AcceptedAt, message.DeliveredAt, message.BouncedAt, message.ComplainedAt, message.FailedAt,
		message.LastErrorClass, message.LastErrorMessage, message.Provider, message.ProviderMessageID,
		message.UpdatedAt,
		message.ID, message.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (w *MessageWriteRepository) MarkProcessing(ctx context.Context, workspaceID, messageID string, now time.Time) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE messages SET status = 'processing', processing_started_at = $1, updated_at = $1
		 WHERE id = $2 AND workspace_id = $3 AND status = 'queued'`,
		now, messageID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (w *MessageWriteRepository) MarkAccepted(ctx context.Context, message domain.Message) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE messages SET status = 'accepted', accepted_at = $1, provider = $2, provider_message_id = $3, updated_at = $4
		 WHERE id = $5 AND workspace_id = $6`,
		message.AcceptedAt, message.Provider, message.ProviderMessageID, message.UpdatedAt,
		message.ID, message.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (w *MessageWriteRepository) MarkDelivered(ctx context.Context, message domain.Message) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE messages SET status = 'delivered', delivered_at = $1, updated_at = $2
		 WHERE id = $3 AND workspace_id = $4`,
		message.DeliveredAt, message.UpdatedAt, message.ID, message.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (w *MessageWriteRepository) MarkBounced(ctx context.Context, message domain.Message) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE messages SET status = 'bounced', bounced_at = $1, last_error_class = $2, last_error_message = $3, updated_at = $4
		 WHERE id = $5 AND workspace_id = $6`,
		message.BouncedAt, message.LastErrorClass, message.LastErrorMessage, message.UpdatedAt,
		message.ID, message.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (w *MessageWriteRepository) MarkComplained(ctx context.Context, message domain.Message) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE messages SET status = 'complained', complained_at = $1, last_error_class = $2, last_error_message = $3, updated_at = $4
		 WHERE id = $5 AND workspace_id = $6`,
		message.ComplainedAt, message.LastErrorClass, message.LastErrorMessage, message.UpdatedAt,
		message.ID, message.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (w *MessageWriteRepository) MarkDelayed(ctx context.Context, message domain.Message) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE messages SET status = 'delayed', last_error_class = $1, last_error_message = $2, updated_at = $3
		 WHERE id = $4 AND workspace_id = $5`,
		message.LastErrorClass, message.LastErrorMessage, message.UpdatedAt,
		message.ID, message.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func (w *MessageWriteRepository) MarkFailed(ctx context.Context, message domain.Message) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE messages SET status = 'failed', failed_at = $1, last_error_class = $2, last_error_message = $3, updated_at = $4
		 WHERE id = $5 AND workspace_id = $6`,
		message.FailedAt, message.LastErrorClass, message.LastErrorMessage, message.UpdatedAt,
		message.ID, message.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

type AttemptReadRepository struct {
	db platformpostgres.DBTX
}

type AttemptWriteRepository struct {
	db platformpostgres.DBTX
}

func NewAttemptReadRepository(db platformpostgres.DBTX) *AttemptReadRepository {
	return &AttemptReadRepository{db: db}
}

func NewAttemptWriteRepository(db platformpostgres.DBTX) *AttemptWriteRepository {
	return &AttemptWriteRepository{db: db}
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

func (r *AttemptReadRepository) ListByMessage(ctx context.Context, workspaceID, messageID string) ([]domain.DeliveryAttempt, error) {
	db := r.getDB(ctx)
	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, message_id, attempt_no, provider, status,
		        request_snapshot, response_snapshot, error_class, error_message,
		        started_at, finished_at
		 FROM delivery_attempts WHERE workspace_id = $1 AND message_id = $2
		 ORDER BY attempt_no`,
		workspaceID, messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.DeliveryAttempt
	for rows.Next() {
		var a domain.DeliveryAttempt
		var reqJSON, respJSON []byte

		if err := rows.Scan(
			&a.ID, &a.WorkspaceID, &a.MessageID, &a.AttemptNo, &a.Provider, &a.Status,
			&reqJSON, &respJSON, &a.ErrorClass, &a.ErrorMessage,
			&a.StartedAt, &a.FinishedAt,
		); err != nil {
			return nil, err
		}

		if reqJSON != nil {
			if err := json.Unmarshal(reqJSON, &a.RequestSnapshot); err != nil {
				return nil, err
			}
		}
		if respJSON != nil {
			if err := json.Unmarshal(respJSON, &a.ResponseSnapshot); err != nil {
				return nil, err
			}
		}

		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []domain.DeliveryAttempt{}
	}

	return results, nil
}

func (r *AttemptReadRepository) NextAttemptNumber(ctx context.Context, workspaceID, messageID string) (int, error) {
	db := r.getDB(ctx)
	var num int
	err := db.QueryRow(ctx,
		`SELECT COALESCE(MAX(attempt_no), 0) + 1 FROM delivery_attempts WHERE workspace_id = $1 AND message_id = $2`,
		workspaceID, messageID,
	).Scan(&num)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func (w *AttemptWriteRepository) Create(ctx context.Context, attempt domain.DeliveryAttempt) error {
	db := w.getDB(ctx)
	reqJSON, err := json.Marshal(attempt.RequestSnapshot)
	if err != nil {
		return err
	}
	respJSON, err := json.Marshal(attempt.ResponseSnapshot)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx,
		`INSERT INTO delivery_attempts (id, workspace_id, message_id, attempt_no, provider, status,
		                                request_snapshot, response_snapshot, error_class, error_message,
		                                started_at, finished_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		attempt.ID, attempt.WorkspaceID, attempt.MessageID, attempt.AttemptNo, attempt.Provider,
		attempt.Status, reqJSON, respJSON, attempt.ErrorClass, attempt.ErrorMessage,
		attempt.StartedAt, attempt.FinishedAt,
	)
	return err
}

func (w *AttemptWriteRepository) Update(ctx context.Context, attempt domain.DeliveryAttempt) error {
	db := w.getDB(ctx)
	reqJSON, err := json.Marshal(attempt.RequestSnapshot)
	if err != nil {
		return err
	}
	respJSON, err := json.Marshal(attempt.ResponseSnapshot)
	if err != nil {
		return err
	}

	tag, err := db.Exec(ctx,
		`UPDATE delivery_attempts SET provider=$1, status=$2, request_snapshot=$3, response_snapshot=$4,
		                              error_class=$5, error_message=$6, finished_at=$7
		 WHERE id=$8 AND workspace_id=$9`,
		attempt.Provider, attempt.Status, reqJSON, respJSON,
		attempt.ErrorClass, attempt.ErrorMessage, attempt.FinishedAt,
		attempt.ID, attempt.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAttemptNotFound
	}
	return nil
}

type RetryStateReadRepository struct {
	db platformpostgres.DBTX
}

type RetryStateWriteRepository struct {
	db platformpostgres.DBTX
}

func NewRetryStateReadRepository(db platformpostgres.DBTX) *RetryStateReadRepository {
	return &RetryStateReadRepository{db: db}
}

func NewRetryStateWriteRepository(db platformpostgres.DBTX) *RetryStateWriteRepository {
	return &RetryStateWriteRepository{db: db}
}

func (r *RetryStateReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (w *RetryStateWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return w.db
}

func (r *RetryStateReadRepository) FindByMessage(ctx context.Context, workspaceID, messageID string) (*domain.RetryState, error) {
	db := r.getDB(ctx)
	var s domain.RetryState
	err := db.QueryRow(ctx,
		`SELECT id, workspace_id, message_id, retry_count, max_retries,
		        next_attempt_at, last_error_class, last_error_message,
		        status, created_at, updated_at
		 FROM retry_states WHERE workspace_id = $1 AND message_id = $2`,
		workspaceID, messageID,
	).Scan(&s.ID, &s.WorkspaceID, &s.MessageID, &s.RetryCount, &s.MaxRetries,
		&s.NextAttemptAt, &s.LastErrorClass, &s.LastErrorMessage,
		&s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (w *RetryStateWriteRepository) Create(ctx context.Context, state domain.RetryState) error {
	db := w.getDB(ctx)
	_, err := db.Exec(ctx,
		`INSERT INTO retry_states (id, workspace_id, message_id, retry_count, max_retries,
		                          next_attempt_at, last_error_class, last_error_message,
		                          status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		state.ID, state.WorkspaceID, state.MessageID, state.RetryCount, state.MaxRetries,
		state.NextAttemptAt, state.LastErrorClass, state.LastErrorMessage,
		state.Status, state.CreatedAt, state.UpdatedAt,
	)
	return err
}

func (w *RetryStateWriteRepository) Update(ctx context.Context, state domain.RetryState) error {
	db := w.getDB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE retry_states SET retry_count=$1, max_retries=$2, next_attempt_at=$3,
		                         last_error_class=$4, last_error_message=$5, status=$6, updated_at=$7
		 WHERE id=$8 AND workspace_id=$9`,
		state.RetryCount, state.MaxRetries, state.NextAttemptAt,
		state.LastErrorClass, state.LastErrorMessage, state.Status, state.UpdatedAt,
		state.ID, state.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAttemptNotFound
	}
	return nil
}

type TransactionalRequestReadRepository struct {
	db platformpostgres.DBTX
}

type TransactionalRequestWriteRepository struct {
	db platformpostgres.DBTX
}

func NewTransactionalRequestReadRepository(db platformpostgres.DBTX) *TransactionalRequestReadRepository {
	return &TransactionalRequestReadRepository{db: db}
}

func NewTransactionalRequestWriteRepository(db platformpostgres.DBTX) *TransactionalRequestWriteRepository {
	return &TransactionalRequestWriteRepository{db: db}
}

func (r *TransactionalRequestReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (w *TransactionalRequestWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return w.db
}

func (r *TransactionalRequestReadRepository) FindByIdempotencyKey(ctx context.Context, workspaceID, idempotencyKey string) (*domain.TransactionalSendRequest, error) {
	db := r.getDB(ctx)
	var req domain.TransactionalSendRequest
	var payloadJSON []byte

	err := db.QueryRow(ctx,
		`SELECT id, workspace_id, idempotency_key, status, request_payload,
		        created_at, updated_at, completed_at, failed_at
		 FROM transactional_send_requests WHERE workspace_id = $1 AND idempotency_key = $2`,
		workspaceID, idempotencyKey,
	).Scan(&req.ID, &req.WorkspaceID, &req.IdempotencyKey, &req.Status, &payloadJSON,
		&req.CreatedAt, &req.UpdatedAt, &req.CompletedAt, &req.FailedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionalRequestNotFound
		}
		return nil, err
	}

	if payloadJSON != nil {
		if err := json.Unmarshal(payloadJSON, &req.RequestPayload); err != nil {
			return nil, err
		}
	}

	return &req, nil
}

func (r *TransactionalRequestReadRepository) FindByID(ctx context.Context, workspaceID, requestID string) (*domain.TransactionalSendRequest, error) {
	db := r.getDB(ctx)
	var req domain.TransactionalSendRequest
	var payloadJSON []byte

	err := db.QueryRow(ctx,
		`SELECT id, workspace_id, idempotency_key, status, request_payload,
		        created_at, updated_at, completed_at, failed_at
		 FROM transactional_send_requests WHERE id = $1 AND workspace_id = $2`,
		requestID, workspaceID,
	).Scan(&req.ID, &req.WorkspaceID, &req.IdempotencyKey, &req.Status, &payloadJSON,
		&req.CreatedAt, &req.UpdatedAt, &req.CompletedAt, &req.FailedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionalRequestNotFound
		}
		return nil, err
	}

	if payloadJSON != nil {
		if err := json.Unmarshal(payloadJSON, &req.RequestPayload); err != nil {
			return nil, err
		}
	}

	return &req, nil
}

func (w *TransactionalRequestWriteRepository) Create(ctx context.Context, request domain.TransactionalSendRequest) error {
	db := w.getDB(ctx)
	payloadJSON, err := json.Marshal(request.RequestPayload)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx,
		`INSERT INTO transactional_send_requests (id, workspace_id, idempotency_key, status,
		                                          request_payload, created_at, updated_at,
		                                          completed_at, failed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		request.ID, request.WorkspaceID, request.IdempotencyKey, request.Status,
		payloadJSON, request.CreatedAt, request.UpdatedAt,
		request.CompletedAt, request.FailedAt,
	)
	if platformpostgres.IsUniqueViolation(err) {
		return domain.ErrIdempotencyKeyConflict
	}
	return err
}

func (w *TransactionalRequestWriteRepository) Update(ctx context.Context, request domain.TransactionalSendRequest) error {
	db := w.getDB(ctx)
	payloadJSON, err := json.Marshal(request.RequestPayload)
	if err != nil {
		return err
	}

	tag, err := db.Exec(ctx,
		`UPDATE transactional_send_requests SET idempotency_key=$1, status=$2, request_payload=$3,
		                                        updated_at=$4, completed_at=$5, failed_at=$6
		 WHERE id=$7 AND workspace_id=$8`,
		request.IdempotencyKey, request.Status, payloadJSON,
		request.UpdatedAt, request.CompletedAt, request.FailedAt,
		request.ID, request.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTransactionalRequestNotFound
	}
	return nil
}

type OutboxRepository struct {
	db platformpostgres.DBTX
}

func NewOutboxRepository(db platformpostgres.DBTX) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
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
		event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload,
		headersJSON, event.WorkspaceID, event.OccurredAt,
	)
	return err
}
