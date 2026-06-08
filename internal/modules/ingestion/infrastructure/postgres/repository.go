package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

type RawEventRepository struct {
	db DBTX
}

type NormalizedEventRepository struct {
	db DBTX
}

func NewRawEventRepository(db DBTX) *RawEventRepository {
	return &RawEventRepository{db: db}
}

func NewNormalizedEventRepository(db DBTX) *NormalizedEventRepository {
	return &NormalizedEventRepository{db: db}
}

func (r *RawEventRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.db
}

func (n *NormalizedEventRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return n.db
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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
		nullable(event.ProviderEventID), nullable(event.ProviderMessageID),
		nullable(event.WorkspaceID), nullable(event.MessageID), nullable(event.EventType),
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
		nullable(event.WorkspaceID), nullable(event.MessageID),
		event.Provider,
		nullable(event.ProviderEventID), nullable(event.ProviderMessageID),
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
	db DBTX
}

func NewOutboxRepository(db DBTX) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
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

type DeliveryMessageResolver struct {
	db DBTX
}

func NewDeliveryMessageResolver(db DBTX) *DeliveryMessageResolver {
	return &DeliveryMessageResolver{db: db}
}

func (r *DeliveryMessageResolver) getDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.db
}

func (r *DeliveryMessageResolver) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error) {
	db := r.getDB(ctx)
	var ref ports.MessageRef
	err := db.QueryRow(ctx,
		`SELECT workspace_id, id, provider, provider_message_id
		 FROM messages WHERE provider = $1 AND provider_message_id = $2`,
		provider, providerMessageID,
	).Scan(&ref.WorkspaceID, &ref.MessageID, &ref.Provider, &ref.ProviderMessageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &ref, nil
}

type IDGenerator func() (string, error)

func mustNewID(gen IDGenerator) string {
	id, err := gen()
	if err != nil {
		panic(err)
	}
	return id
}

func sanitizeHeaders(headers map[string][]string) map[string][]string {
	safe := make(map[string][]string, len(headers))
	for k, v := range headers {
		kl := lowerHeader(k)
		switch kl {
		case "content-type", "user-agent", "x-sendflow-fake-signature",
			"x-amz-sns-message-type", "x-amz-sns-message-id",
			"x-amz-sns-topic-arn", "x-amz-sns-subscription-arn",
			"x-forwarded-for", "x-forwarded-proto", "x-real-ip":
			safe[k] = v
		}
	}
	return safe
}

func lowerHeader(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}
