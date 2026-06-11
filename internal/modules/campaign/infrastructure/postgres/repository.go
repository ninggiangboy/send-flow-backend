package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type CampaignReadRepository struct {
	db platformpostgres.DBTX
}

type CampaignWriteRepository struct {
	db platformpostgres.DBTX
}

func NewCampaignReadRepository(db platformpostgres.DBTX) *CampaignReadRepository {
	return &CampaignReadRepository{db: db}
}

func NewCampaignWriteRepository(db platformpostgres.DBTX) *CampaignWriteRepository {
	return &CampaignWriteRepository{db: db}
}

func (r *CampaignReadRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (w *CampaignWriteRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return w.db
}

func (r *CampaignReadRepository) FindByID(ctx context.Context, workspaceID, campaignID string) (*domain.Campaign, error) {
	db := r.getDB(ctx)
	var c domain.Campaign
	var audienceType, audienceID string
	var audienceContactIDsJSON []byte
	var templateID, templateVersionID *string
	var scheduledAt *time.Time
	var cancelledAt, pausedAt, completedAt *time.Time

	err := db.QueryRow(ctx,
		`SELECT id, workspace_id, name, status, audience_type, COALESCE(audience_id, ''), audience_contact_ids,
		        template_id, template_version_id, sender_domain_id, message_type,
		        scheduled_at, planned_recipients, created_at, updated_at,
		        cancelled_at, paused_at, completed_at
		 FROM campaigns WHERE id = $1 AND workspace_id = $2`,
		campaignID, workspaceID,
	).Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Status, &audienceType, &audienceID,
		&audienceContactIDsJSON, &templateID, &templateVersionID,
		&c.SenderDomainID, &c.MessageType, &scheduledAt, &c.PlannedRecipients,
		&c.CreatedAt, &c.UpdatedAt, &cancelledAt, &pausedAt, &completedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCampaignNotFound
		}
		return nil, err
	}

	c.AudienceRef = domain.AudienceRef{
		Type: domain.AudienceType(audienceType),
		ID:   stringOrZero(audienceID),
	}
	if audienceContactIDsJSON != nil {
		if err := json.Unmarshal(audienceContactIDsJSON, &c.AudienceRef.ContactIDs); err != nil {
			return nil, err
		}
	}
	if templateID != nil {
		c.TemplateRef.TemplateID = *templateID
	}
	if templateVersionID != nil {
		c.TemplateRef.TemplateVersionID = *templateVersionID
	}
	c.ScheduledAt = scheduledAt
	c.CancelledAt = cancelledAt
	c.PausedAt = pausedAt
	c.CompletedAt = completedAt

	return &c, nil
}

func (r *CampaignReadRepository) List(ctx context.Context, query ports.CampaignListQuery) ([]domain.Campaign, string, error) {
	db := r.getDB(ctx)
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if query.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.Status)
		argIdx++
	}
	if query.SenderDomainID != "" {
		where += " AND sender_domain_id = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.SenderDomainID)
		argIdx++
	}
	if query.TemplateID != "" {
		where += " AND template_id = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.TemplateID)
		argIdx++
	}
	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM campaigns WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, name, status, audience_type, COALESCE(audience_id, ''), audience_contact_ids,
		        COALESCE(template_id, ''), template_version_id, sender_domain_id, message_type,
		        scheduled_at, planned_recipients, created_at, updated_at,
		        cancelled_at, paused_at, completed_at
		 FROM campaigns `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.Campaign
	for rows.Next() {
		var c domain.Campaign
		var audienceType, audienceID, templateID string
		var audienceContactIDsJSON []byte
		var templateVersionID *string
		var scheduledAt *time.Time
		var cancelledAt, pausedAt, completedAt *time.Time

		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Status, &audienceType, &audienceID,
			&audienceContactIDsJSON, &templateID, &templateVersionID,
			&c.SenderDomainID, &c.MessageType, &scheduledAt, &c.PlannedRecipients,
			&c.CreatedAt, &c.UpdatedAt, &cancelledAt, &pausedAt, &completedAt); err != nil {
			return nil, "", err
		}

		c.AudienceRef = domain.AudienceRef{
			Type: domain.AudienceType(audienceType),
			ID:   audienceID,
		}
		if audienceContactIDsJSON != nil {
			if err := json.Unmarshal(audienceContactIDsJSON, &c.AudienceRef.ContactIDs); err != nil {
				return nil, "", err
			}
		}
		c.TemplateRef = domain.TemplateRef{TemplateID: templateID}
		if templateVersionID != nil {
			c.TemplateRef.TemplateVersionID = *templateVersionID
		}
		c.ScheduledAt = scheduledAt
		c.CancelledAt = cancelledAt
		c.PausedAt = pausedAt
		c.CompletedAt = completedAt

		results = append(results, c)
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
		results = []domain.Campaign{}
	}

	return results, nextCursor, nil
}

func (r *CampaignReadRepository) ListCandidates(ctx context.Context, query ports.CandidateListQuery) ([]domain.CampaignMessageCandidate, string, error) {
	db := r.getDB(ctx)
	args := []any{query.WorkspaceID, query.CampaignID}
	where := "WHERE workspace_id = $1 AND campaign_id = $2"
	argIdx := 3

	if query.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.Status)
		argIdx++
	}
	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM campaign_message_candidates WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, campaign_id, contact_id, email_normalized, recipient_snapshot, status, created_at, updated_at
		 FROM campaign_message_candidates `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.CampaignMessageCandidate
	for rows.Next() {
		var cand domain.CampaignMessageCandidate
		var snapshotJSON []byte

		if err := rows.Scan(&cand.ID, &cand.WorkspaceID, &cand.CampaignID, &cand.ContactID,
			&cand.EmailNormalized, &snapshotJSON, &cand.Status,
			&cand.CreatedAt, &cand.UpdatedAt); err != nil {
			return nil, "", err
		}
		if snapshotJSON != nil {
			if err := json.Unmarshal(snapshotJSON, &cand.RecipientSnapshot); err != nil {
				return nil, "", err
			}
		}
		results = append(results, cand)
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
		results = []domain.CampaignMessageCandidate{}
	}

	return results, nextCursor, nil
}

func (r *CampaignReadRepository) CountCandidates(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	db := r.getDB(ctx)
	var count int64
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM campaign_message_candidates WHERE workspace_id = $1 AND campaign_id = $2`,
		workspaceID, campaignID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (w *CampaignWriteRepository) Create(ctx context.Context, c domain.Campaign) error {
	db := w.getDB(ctx)
	audienceContactIDsJSON, err := json.Marshal(c.AudienceRef.ContactIDs)
	if err != nil {
		return err
	}

	var templateVersionID *string
	if c.TemplateRef.TemplateVersionID != "" {
		templateVersionID = &c.TemplateRef.TemplateVersionID
	}

	_, err = db.Exec(ctx,
		`INSERT INTO campaigns (id, workspace_id, name, status, audience_type, audience_id, audience_contact_ids,
		                        template_id, template_version_id, sender_domain_id, message_type,
		                        scheduled_at, planned_recipients, created_at, updated_at,
		                        cancelled_at, paused_at, completed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		c.ID, c.WorkspaceID, c.Name, string(c.Status), string(c.AudienceRef.Type),
		platformpostgres.Nullable(c.AudienceRef.ID), audienceContactIDsJSON,
		c.TemplateRef.TemplateID, templateVersionID, c.SenderDomainID, string(c.MessageType),
		c.ScheduledAt, c.PlannedRecipients, c.CreatedAt, c.UpdatedAt,
		c.CancelledAt, c.PausedAt, c.CompletedAt,
	)
	return err
}

func (w *CampaignWriteRepository) Update(ctx context.Context, c domain.Campaign) error {
	db := w.getDB(ctx)
	audienceContactIDsJSON, err := json.Marshal(c.AudienceRef.ContactIDs)
	if err != nil {
		return err
	}

	var templateVersionID *string
	if c.TemplateRef.TemplateVersionID != "" {
		templateVersionID = &c.TemplateRef.TemplateVersionID
	}

	tag, err := db.Exec(ctx,
		`UPDATE campaigns SET name=$1, status=$2, audience_type=$3, audience_id=$4, audience_contact_ids=$5,
		                      template_id=$6, template_version_id=$7, sender_domain_id=$8, message_type=$9,
		                      scheduled_at=$10, planned_recipients=$11, updated_at=$12,
		                      cancelled_at=$13, paused_at=$14, completed_at=$15
		 WHERE id=$16 AND workspace_id=$17`,
		c.Name, string(c.Status), string(c.AudienceRef.Type), platformpostgres.Nullable(c.AudienceRef.ID),
		audienceContactIDsJSON, c.TemplateRef.TemplateID, templateVersionID,
		c.SenderDomainID, string(c.MessageType),
		c.ScheduledAt, c.PlannedRecipients, c.UpdatedAt,
		c.CancelledAt, c.PausedAt, c.CompletedAt,
		c.ID, c.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCampaignNotFound
	}
	return nil
}

func (w *CampaignWriteRepository) ReplaceCandidates(ctx context.Context, workspaceID, campaignID string, candidates []domain.CampaignMessageCandidate) error {
	db := w.getDB(ctx)
	if _, err := db.Exec(ctx,
		`DELETE FROM campaign_message_candidates WHERE workspace_id = $1 AND campaign_id = $2`,
		workspaceID, campaignID); err != nil {
		return err
	}

	if len(candidates) == 0 {
		return nil
	}

	values := make([]string, 0, len(candidates))
	args := make([]any, 0, len(candidates)*9)
	idx := 1

	for _, cand := range candidates {
		snapshotJSON, err := json.Marshal(cand.RecipientSnapshot)
		if err != nil {
			return err
		}
		values = append(values, "($"+
			platformpostgres.Itoa(idx)+",$"+platformpostgres.Itoa(idx+1)+",$"+platformpostgres.Itoa(idx+2)+",$"+platformpostgres.Itoa(idx+3)+",$"+platformpostgres.Itoa(idx+4)+",$"+platformpostgres.Itoa(idx+5)+",$"+platformpostgres.Itoa(idx+6)+",$"+platformpostgres.Itoa(idx+7)+",$"+platformpostgres.Itoa(idx+8)+")")
		args = append(args, cand.ID, cand.WorkspaceID, cand.CampaignID, cand.ContactID,
			cand.EmailNormalized, snapshotJSON, string(cand.Status), cand.CreatedAt, cand.UpdatedAt)
		idx += 9
	}

	_, err := db.Exec(ctx,
		`INSERT INTO campaign_message_candidates (id, workspace_id, campaign_id, contact_id, email_normalized, recipient_snapshot, status, created_at, updated_at) VALUES `+
			strings.Join(values, ","), args...)
	return err
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

func stringOrZero(s string) string {
	return s
}
