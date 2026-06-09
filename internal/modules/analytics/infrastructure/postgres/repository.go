package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type txKey struct{}

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func getDB(ctx context.Context, db DBTX) DBTX {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if ok {
		return tx
	}
	return db
}

type EventFactRepository struct {
	db DBTX
}

func NewEventFactRepository(db DBTX) *EventFactRepository {
	return &EventFactRepository{db: db}
}

func (r *EventFactRepository) Create(ctx context.Context, fact domain.EmailEventFact) error {
	md, err := json.Marshal(fact.MetadataJSON)
	if err != nil {
		return err
	}

	db := getDB(ctx, r.db)
	_, err = db.Exec(ctx,
		`INSERT INTO analytics_event_facts
		 (id, source_event_id, source_event_type, workspace_id, campaign_id, message_id,
		  provider, provider_message_id, provider_event_id, event_type, recipient_domain,
		  occurred_at, received_at, metadata_json, created_at)
		 VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),
		         NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,NULLIF($11,''),
		         $12,$13,$14::jsonb,$15)`,
		fact.ID, fact.SourceEventID, fact.SourceEventType,
		fact.WorkspaceID, fact.CampaignID, fact.MessageID,
		fact.Provider, fact.ProviderMessageID, fact.ProviderEventID,
		fact.EventType, fact.RecipientDomain,
		fact.OccurredAt, fact.ReceivedAt, string(md), fact.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAnalyticsEventDuplicate
		}
		return err
	}
	return nil
}

func (r *EventFactRepository) FindBySourceEventID(ctx context.Context, sourceEventID string) (*domain.EmailEventFact, error) {
	db := getDB(ctx, r.db)
	var fact domain.EmailEventFact
	var campaignID, messageID, provider, providerMessageID, providerEventID, recipientDomain *string
	var md []byte

	err := db.QueryRow(ctx,
		`SELECT id, source_event_id, source_event_type, workspace_id, campaign_id, message_id,
		        provider, provider_message_id, provider_event_id, event_type, recipient_domain,
		        occurred_at, received_at, metadata_json, created_at
		 FROM analytics_event_facts WHERE source_event_id = $1`,
		sourceEventID,
	).Scan(&fact.ID, &fact.SourceEventID, &fact.SourceEventType, &fact.WorkspaceID,
		&campaignID, &messageID, &provider, &providerMessageID, &providerEventID,
		&fact.EventType, &recipientDomain, &fact.OccurredAt, &fact.ReceivedAt,
		&md, &fact.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAnalyticsProjectionNotFound
		}
		return nil, err
	}

	if campaignID != nil {
		fact.CampaignID = *campaignID
	}
	if messageID != nil {
		fact.MessageID = *messageID
	}
	if provider != nil {
		fact.Provider = *provider
	}
	if providerMessageID != nil {
		fact.ProviderMessageID = *providerMessageID
	}
	if providerEventID != nil {
		fact.ProviderEventID = *providerEventID
	}
	if recipientDomain != nil {
		fact.RecipientDomain = *recipientDomain
	}
	if md != nil {
		json.Unmarshal(md, &fact.MetadataJSON)
	}

	return &fact, nil
}

type ProjectionRepository struct {
	db DBTX
}

func NewProjectionRepository(db DBTX) *ProjectionRepository {
	return &ProjectionRepository{db: db}
}

func (r *ProjectionRepository) IncrementWorkspaceOverview(ctx context.Context, workspaceID string, eventType string, occurredAt time.Time) error {
	col := eventTypeToCounter(eventType)
	if col == "" {
		return nil
	}

	db := getDB(ctx, r.db)
	_, err := db.Exec(ctx,
		`INSERT INTO workspace_analytics_overviews (workspace_id, `+col+`, last_event_at, last_updated_at)
		 VALUES ($1, 1, $2, NOW())
		 ON CONFLICT (workspace_id) DO UPDATE SET
		   `+col+` = workspace_analytics_overviews.`+col+` + 1,
		   last_event_at = GREATEST(workspace_analytics_overviews.last_event_at, $2),
		   last_updated_at = NOW()`,
		workspaceID, occurredAt,
	)
	return err
}

func (r *ProjectionRepository) IncrementCampaignSummary(ctx context.Context, workspaceID, campaignID string, eventType string, occurredAt time.Time) error {
	col := eventTypeToCounter(eventType)
	if col == "" {
		return nil
	}

	db := getDB(ctx, r.db)
	_, err := db.Exec(ctx,
		`INSERT INTO campaign_delivery_summaries (workspace_id, campaign_id, `+col+`, last_event_at, last_updated_at)
		 VALUES ($1, $2, 1, $3, NOW())
		 ON CONFLICT (workspace_id, campaign_id) DO UPDATE SET
		   `+col+` = campaign_delivery_summaries.`+col+` + 1,
		   last_event_at = GREATEST(campaign_delivery_summaries.last_event_at, $3),
		   last_updated_at = NOW()`,
		workspaceID, campaignID, occurredAt,
	)
	return err
}

func (r *ProjectionRepository) IncrementDeliverability(ctx context.Context, workspaceID, provider, recipientDomain, eventType string, occurredAt time.Time) error {
	col := deliverabilityCounter(eventType)
	if col == "" {
		return nil
	}

	db := getDB(ctx, r.db)
	_, err := db.Exec(ctx,
		`INSERT INTO deliverability_projections (workspace_id, provider, recipient_domain, `+col+`, last_event_at, last_updated_at)
		 VALUES ($1, $2, $3, 1, $4, NOW())
		 ON CONFLICT (workspace_id, provider, recipient_domain) DO UPDATE SET
		   `+col+` = deliverability_projections.`+col+` + 1,
		   last_event_at = GREATEST(deliverability_projections.last_event_at, $4),
		   last_updated_at = NOW()`,
		workspaceID, provider, recipientDomain, occurredAt,
	)
	return err
}

func (r *ProjectionRepository) GetWorkspaceOverview(ctx context.Context, workspaceID string) (*domain.WorkspaceAnalyticsOverview, error) {
	db := getDB(ctx, r.db)
	var overview domain.WorkspaceAnalyticsOverview
	err := db.QueryRow(ctx,
		`SELECT workspace_id, queued_count, accepted_count, delivered_count, bounced_count,
		        complained_count, opened_count, clicked_count, unsubscribed_count, retry_scheduled_count,
		        last_event_at, last_updated_at
		 FROM workspace_analytics_overviews WHERE workspace_id = $1`,
		workspaceID,
	).Scan(&overview.WorkspaceID, &overview.QueuedCount, &overview.AcceptedCount,
		&overview.DeliveredCount, &overview.BouncedCount, &overview.ComplainedCount,
		&overview.OpenedCount, &overview.ClickedCount, &overview.UnsubscribedCount,
		&overview.RetryScheduledCount, &overview.LastEventAt, &overview.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAnalyticsProjectionNotFound
		}
		return nil, err
	}
	return &overview, nil
}

func (r *ProjectionRepository) GetCampaignSummary(ctx context.Context, workspaceID, campaignID string) (*domain.CampaignDeliverySummary, error) {
	db := getDB(ctx, r.db)
	var summary domain.CampaignDeliverySummary
	err := db.QueryRow(ctx,
		`SELECT workspace_id, campaign_id, queued_count, accepted_count, delivered_count, bounced_count,
		        complained_count, opened_count, clicked_count, unsubscribed_count, retry_scheduled_count,
		        last_event_at, last_updated_at
		 FROM campaign_delivery_summaries WHERE workspace_id = $1 AND campaign_id = $2`,
		workspaceID, campaignID,
	).Scan(&summary.WorkspaceID, &summary.CampaignID, &summary.QueuedCount,
		&summary.AcceptedCount, &summary.DeliveredCount, &summary.BouncedCount,
		&summary.ComplainedCount, &summary.OpenedCount, &summary.ClickedCount,
		&summary.UnsubscribedCount, &summary.RetryScheduledCount,
		&summary.LastEventAt, &summary.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAnalyticsProjectionNotFound
		}
		return nil, err
	}
	return &summary, nil
}

func (r *ProjectionRepository) ListDeliverability(ctx context.Context, workspaceID string, filter domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error) {
	db := getDB(ctx, r.db)
	query := `SELECT workspace_id, provider, recipient_domain,
	                 delivered_count, bounced_count, complained_count, opened_count, clicked_count,
	                 last_event_at, last_updated_at
	          FROM deliverability_projections
	          WHERE workspace_id = $1`
	args := []any{workspaceID}
	argIdx := 2

	if filter.Provider != "" {
		query += ` AND provider = $` + string(rune('0'+argIdx))
		args = append(args, filter.Provider)
		argIdx++
	}
	if filter.RecipientDomain != "" {
		query += ` AND recipient_domain = $` + string(rune('0'+argIdx))
		args = append(args, filter.RecipientDomain)
		argIdx++
	}
	query += ` ORDER BY provider, recipient_domain`

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projections []domain.DeliverabilityProjection
	for rows.Next() {
		var p domain.DeliverabilityProjection
		if err := rows.Scan(&p.WorkspaceID, &p.Provider, &p.RecipientDomain,
			&p.DeliveredCount, &p.BouncedCount, &p.ComplainedCount,
			&p.OpenedCount, &p.ClickedCount, &p.LastEventAt, &p.LastUpdatedAt,
		); err != nil {
			return nil, err
		}
		projections = append(projections, p)
	}
	return projections, rows.Err()
}

func eventTypeToCounter(eventType string) string {
	switch eventType {
	case domain.EventTypeQueued:
		return "queued_count"
	case domain.EventTypeAccepted:
		return "accepted_count"
	case domain.EventTypeDelivered:
		return "delivered_count"
	case domain.EventTypeBounced:
		return "bounced_count"
	case domain.EventTypeComplained:
		return "complained_count"
	case domain.EventTypeOpened:
		return "opened_count"
	case domain.EventTypeClicked:
		return "clicked_count"
	case domain.EventTypeUnsubscribed:
		return "unsubscribed_count"
	case domain.EventTypeRetryScheduled:
		return "retry_scheduled_count"
	default:
		return ""
	}
}

func deliverabilityCounter(eventType string) string {
	switch eventType {
	case domain.EventTypeDelivered:
		return "delivered_count"
	case domain.EventTypeBounced:
		return "bounced_count"
	case domain.EventTypeComplained:
		return "complained_count"
	case domain.EventTypeOpened:
		return "opened_count"
	case domain.EventTypeClicked:
		return "clicked_count"
	default:
		return ""
	}
}

func (r *EventFactRepository) WithTx(tx pgx.Tx) *EventFactRepository {
	return &EventFactRepository{db: tx}
}

func (r *ProjectionRepository) WithTx(tx pgx.Tx) *ProjectionRepository {
	return &ProjectionRepository{db: tx}
}

type TransactionManager struct {
	pool DBTX
}

func NewTransactionManager(pool DBTX) *TransactionManager {
	return &TransactionManager{pool: pool}
}

func (tm *TransactionManager) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	conn, ok := tm.pool.(interface {
		Begin(ctx context.Context) (pgx.Tx, error)
	})
	if !ok {
		return errors.New("transaction manager requires a pool or conn that supports Begin")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type OutboxRepository struct {
	db DBTX
}

func NewOutboxRepository(db DBTX) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Save(ctx context.Context, event ports.OutboxEvent) error {
	db := getDB(ctx, r.db)
	_, err := db.Exec(ctx,
		`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, workspace_id, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		event.ID, event.AggregateType, event.AggregateID, event.EventType,
		event.Payload, event.WorkspaceID, event.OccurredAt,
	)
	return err
}
