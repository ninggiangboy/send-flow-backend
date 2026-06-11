package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/httpheaders"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type RawEventRepository struct {
	db platformpostgres.DBTX
}

type NormalizedEventRepository struct {
	db platformpostgres.DBTX
}

func NewRawEventRepository(db platformpostgres.DBTX) *RawEventRepository {
	return &RawEventRepository{db: db}
}

func NewNormalizedEventRepository(db platformpostgres.DBTX) *NormalizedEventRepository {
	return &NormalizedEventRepository{db: db}
}

func (r *RawEventRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (n *NormalizedEventRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return n.db
}

func (r *RawEventRepository) FindByID(ctx context.Context, id string) (*domain.ProviderWebhookEvent, error) {
	db := r.getDB(ctx)
	var e domain.ProviderWebhookEvent
	var workspaceID, messageID, providerEventID, providerMessageID, eventType *string

	err := db.QueryRow(ctx,
		`SELECT id, provider, provider_event_id, provider_message_id,
		        workspace_id, message_id, event_type,
		        payload_json, headers_json, signature_valid,
		        received_at, created_at
		 FROM provider_webhook_events WHERE id = $1`, id,
	).Scan(&e.ID, &e.Provider, &providerEventID, &providerMessageID,
		&workspaceID, &messageID, &eventType,
		&e.PayloadJSON, &e.HeadersJSON, &e.SignatureValid,
		&e.ReceivedAt, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRawEventNotFound
		}
		return nil, err
	}

	if workspaceID != nil {
		e.WorkspaceID = *workspaceID
	}
	if messageID != nil {
		e.MessageID = *messageID
	}
	if providerEventID != nil {
		e.ProviderEventID = *providerEventID
	}
	if providerMessageID != nil {
		e.ProviderMessageID = *providerMessageID
	}
	if eventType != nil {
		e.EventType = *eventType
	}

	return &e, nil
}

func (r *RawEventRepository) FindByProviderEventID(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
	db := r.getDB(ctx)
	var e domain.ProviderWebhookEvent
	var workspaceID, messageID, providerMessageID, eventType *string

	err := db.QueryRow(ctx,
		`SELECT id, provider, provider_event_id, provider_message_id,
		        workspace_id, message_id, event_type,
		        payload_json, headers_json, signature_valid,
		        received_at, created_at
		 FROM provider_webhook_events
		 WHERE provider = $1 AND provider_event_id = $2`, provider, providerEventID,
	).Scan(&e.ID, &e.Provider, &e.ProviderEventID, &providerMessageID,
		&workspaceID, &messageID, &eventType,
		&e.PayloadJSON, &e.HeadersJSON, &e.SignatureValid,
		&e.ReceivedAt, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRawEventNotFound
		}
		return nil, err
	}

	if workspaceID != nil {
		e.WorkspaceID = *workspaceID
	}
	if messageID != nil {
		e.MessageID = *messageID
	}
	if providerMessageID != nil {
		e.ProviderMessageID = *providerMessageID
	}
	if eventType != nil {
		e.EventType = *eventType
	}

	return &e, nil
}

func (r *RawEventRepository) Create(ctx context.Context, event domain.ProviderWebhookEvent) error {
	db := r.getDB(ctx)
	tag, err := db.Exec(ctx,
		`INSERT INTO provider_webhook_events
		 (id, provider, provider_event_id, provider_message_id,
		  workspace_id, message_id, event_type,
		  payload_json, headers_json, signature_valid,
		  received_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 ON CONFLICT (provider, provider_event_id) WHERE provider_event_id IS NOT NULL DO NOTHING`,
		event.ID, event.Provider,
		platformpostgres.Nullable(event.ProviderEventID), platformpostgres.Nullable(event.ProviderMessageID),
		platformpostgres.Nullable(event.WorkspaceID), platformpostgres.Nullable(event.MessageID), platformpostgres.Nullable(event.EventType),
		event.PayloadJSON, event.HeadersJSON, event.SignatureValid,
		event.ReceivedAt, event.CreatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 && event.ProviderEventID != "" {
		return domain.ErrDuplicateEventConflict
	}
	return nil
}

func (n *NormalizedEventRepository) FindByID(ctx context.Context, id string) (*domain.NormalizedProviderEvent, error) {
	db := n.getDB(ctx)
	var e domain.NormalizedProviderEvent
	var workspaceID, messageID, providerEventID, providerMessageID *string

	err := db.QueryRow(ctx,
		`SELECT id, raw_event_id, workspace_id, message_id, provider,
		        provider_event_id, provider_message_id, event_type,
		        occurred_at, received_at, payload_json, created_at
		 FROM normalized_provider_events WHERE id = $1`, id,
	).Scan(&e.ID, &e.RawEventID, &workspaceID, &messageID, &e.Provider,
		&providerEventID, &providerMessageID, &e.EventType,
		&e.OccurredAt, &e.ReceivedAt, &e.PayloadJSON, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNormalizedEventNotFound
		}
		return nil, err
	}

	if workspaceID != nil {
		e.WorkspaceID = *workspaceID
	}
	if messageID != nil {
		e.MessageID = *messageID
	}
	if providerEventID != nil {
		e.ProviderEventID = *providerEventID
	}
	if providerMessageID != nil {
		e.ProviderMessageID = *providerMessageID
	}

	return &e, nil
}

func (n *NormalizedEventRepository) FindByProviderEventID(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error) {
	db := n.getDB(ctx)
	var e domain.NormalizedProviderEvent
	var workspaceID, messageID, providerMessageID *string

	err := db.QueryRow(ctx,
		`SELECT id, raw_event_id, workspace_id, message_id, provider,
		        provider_event_id, provider_message_id, event_type,
		        occurred_at, received_at, payload_json, created_at
		 FROM normalized_provider_events
		 WHERE provider = $1 AND provider_event_id = $2 AND event_type = $3`,
		provider, providerEventID, eventType,
	).Scan(&e.ID, &e.RawEventID, &workspaceID, &messageID, &e.Provider,
		&e.ProviderEventID, &providerMessageID, &e.EventType,
		&e.OccurredAt, &e.ReceivedAt, &e.PayloadJSON, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNormalizedEventNotFound
		}
		return nil, err
	}

	if workspaceID != nil {
		e.WorkspaceID = *workspaceID
	}
	if messageID != nil {
		e.MessageID = *messageID
	}
	if providerMessageID != nil {
		e.ProviderMessageID = *providerMessageID
	}

	return &e, nil
}

func (n *NormalizedEventRepository) Create(ctx context.Context, event domain.NormalizedProviderEvent) error {
	db := n.getDB(ctx)
	tag, err := db.Exec(ctx,
		`INSERT INTO normalized_provider_events
		 (id, raw_event_id, workspace_id, message_id, provider,
		  provider_event_id, provider_message_id, event_type,
		  occurred_at, received_at, payload_json, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 ON CONFLICT (provider, provider_event_id, event_type) WHERE provider_event_id IS NOT NULL DO NOTHING`,
		event.ID, event.RawEventID,
		platformpostgres.Nullable(event.WorkspaceID), platformpostgres.Nullable(event.MessageID),
		event.Provider,
		platformpostgres.Nullable(event.ProviderEventID), platformpostgres.Nullable(event.ProviderMessageID),
		event.EventType, event.OccurredAt, event.ReceivedAt,
		event.PayloadJSON, event.CreatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 && event.ProviderEventID != "" {
		return domain.ErrDuplicateEventConflict
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

func sanitizeHeaders(headers map[string][]string) map[string][]string {
	sanitized := httpheaders.SanitizeHeaders(headers,
		"content-type", "user-agent", "x-sendflow-fake-signature",
		"x-amz-sns-message-type", "x-amz-sns-message-id",
		"x-amz-sns-topic-arn", "x-amz-sns-subscription-arn",
		"x-forwarded-for", "x-forwarded-proto", "x-real-ip",
	)
	result := make(map[string][]string, len(sanitized))
	for k, v := range sanitized {
		result[k] = []string{v}
	}
	return result
}

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}
