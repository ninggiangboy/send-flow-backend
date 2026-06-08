package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
)

type txKey struct{}

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type TrackingLinkRepository struct {
	db DBTX
}

type TrackingEventRepository struct {
	db DBTX
}

func NewTrackingLinkRepository(db DBTX) *TrackingLinkRepository {
	return &TrackingLinkRepository{db: db}
}

func NewTrackingEventRepository(db DBTX) *TrackingEventRepository {
	return &TrackingEventRepository{db: db}
}

func (r *TrackingLinkRepository) FindByID(ctx context.Context, trackingID string) (*domain.TrackingLink, error) {
	var link domain.TrackingLink
	var metadataJSON []byte
	var expiresAt *time.Time

	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, message_id, destination_url, link_type, metadata_json, created_at, expires_at
		 FROM tracking_links WHERE id = $1`,
		trackingID,
	).Scan(&link.ID, &link.WorkspaceID, &link.MessageID, &link.DestinationURL,
		&link.LinkType, &metadataJSON, &link.CreatedAt, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTrackingLinkNotFound
		}
		return nil, err
	}

	if expiresAt != nil {
		link.ExpiresAt = expiresAt
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &link.MetadataJSON); err != nil {
			link.MetadataJSON = map[string]any{}
		}
	} else {
		link.MetadataJSON = map[string]any{}
	}

	return &link, nil
}

func (r *TrackingLinkRepository) ListByMessage(ctx context.Context, workspaceID, messageID string) ([]domain.TrackingLink, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, message_id, destination_url, link_type, metadata_json, created_at, expires_at
		 FROM tracking_links WHERE workspace_id = $1 AND message_id = $2
		 ORDER BY created_at DESC`,
		workspaceID, messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []domain.TrackingLink
	for rows.Next() {
		var link domain.TrackingLink
		var metadataJSON []byte
		var expiresAt *time.Time

		if err := rows.Scan(&link.ID, &link.WorkspaceID, &link.MessageID, &link.DestinationURL,
			&link.LinkType, &metadataJSON, &link.CreatedAt, &expiresAt); err != nil {
			return nil, err
		}

		if expiresAt != nil {
			link.ExpiresAt = expiresAt
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &link.MetadataJSON); err != nil {
				link.MetadataJSON = map[string]any{}
			}
		} else {
			link.MetadataJSON = map[string]any{}
		}

		links = append(links, link)
	}

	if links == nil {
		links = []domain.TrackingLink{}
	}
	return links, rows.Err()
}

func (r *TrackingLinkRepository) Create(ctx context.Context, link domain.TrackingLink) error {
	metadataJSON, err := json.Marshal(link.MetadataJSON)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO tracking_links (id, workspace_id, message_id, destination_url, link_type, metadata_json, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)`,
		link.ID, link.WorkspaceID, link.MessageID, link.DestinationURL,
		link.LinkType, metadataJSON, link.CreatedAt, link.ExpiresAt,
	)
	return err
}

func (r *TrackingEventRepository) FindBySourceEvent(ctx context.Context, source, sourceEventID, eventType string) (*domain.TrackingEvent, error) {
	var evt domain.TrackingEvent
	var metadataJSON []byte
	var trackingLinkID, provider, providerEventID, providerMessageID *string

	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, message_id, tracking_link_id, event_type, source,
		        source_event_id, provider, provider_event_id, provider_message_id,
		        occurred_at, received_at, metadata_json, created_at
		 FROM tracking_events
		 WHERE source = $1 AND source_event_id = $2 AND event_type = $3`,
		source, sourceEventID, eventType,
	).Scan(&evt.ID, &evt.WorkspaceID, &evt.MessageID, &trackingLinkID,
		&evt.EventType, &evt.Source, &evt.SourceEventID,
		&provider, &providerEventID, &providerMessageID,
		&evt.OccurredAt, &evt.ReceivedAt, &metadataJSON, &evt.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if trackingLinkID != nil {
		evt.TrackingLinkID = *trackingLinkID
	}
	if provider != nil {
		evt.Provider = *provider
	}
	if providerEventID != nil {
		evt.ProviderEventID = *providerEventID
	}
	if providerMessageID != nil {
		evt.ProviderMessageID = *providerMessageID
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &evt.MetadataJSON); err != nil {
			evt.MetadataJSON = map[string]any{}
		}
	} else {
		evt.MetadataJSON = map[string]any{}
	}

	return &evt, nil
}

func (r *TrackingEventRepository) Create(ctx context.Context, evt domain.TrackingEvent) error {
	metadataJSON, err := json.Marshal(evt.MetadataJSON)
	if err != nil {
		return err
	}

	var trackingLinkID, provider, providerEventID, providerMessageID *string
	if evt.TrackingLinkID != "" {
		trackingLinkID = &evt.TrackingLinkID
	}
	if evt.Provider != "" {
		provider = &evt.Provider
	}
	if evt.ProviderEventID != "" {
		providerEventID = &evt.ProviderEventID
	}
	if evt.ProviderMessageID != "" {
		providerMessageID = &evt.ProviderMessageID
	}

	db := r.getDB(ctx)
	_, err = db.Exec(ctx,
		`INSERT INTO tracking_events (id, workspace_id, message_id, tracking_link_id, event_type, source,
		 source_event_id, provider, provider_event_id, provider_message_id,
		 occurred_at, received_at, metadata_json, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14)`,
		evt.ID, evt.WorkspaceID, evt.MessageID, trackingLinkID,
		evt.EventType, evt.Source, nullable(evt.SourceEventID),
		provider, providerEventID, providerMessageID,
		evt.OccurredAt, evt.ReceivedAt, metadataJSON, evt.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrTrackingEventConflict
		}
		return err
	}
	return nil
}

func (r *TrackingEventRepository) ListByMessage(ctx context.Context, workspaceID, messageID string, limit int, cursor string) ([]domain.TrackingEvent, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = r.db.Query(ctx,
			`SELECT id, workspace_id, message_id, tracking_link_id, event_type, source,
			        source_event_id, provider, provider_event_id, provider_message_id,
			        occurred_at, received_at, metadata_json, created_at
			 FROM tracking_events
			 WHERE workspace_id = $1 AND message_id = $2
			 ORDER BY occurred_at DESC, id DESC
			 LIMIT $3`,
			workspaceID, messageID, limit+1,
		)
	} else {
		var cursorOccurredAt time.Time
		var cursorID string
		if err := parseCursor(cursor, &cursorOccurredAt, &cursorID); err != nil {
			return nil, "", err
		}
		rows, err = r.db.Query(ctx,
			`SELECT id, workspace_id, message_id, tracking_link_id, event_type, source,
			        source_event_id, provider, provider_event_id, provider_message_id,
			        occurred_at, received_at, metadata_json, created_at
			 FROM tracking_events
			 WHERE workspace_id = $1 AND message_id = $2
			   AND (occurred_at, id) < ($3, $4)
			 ORDER BY occurred_at DESC, id DESC
			 LIMIT $5`,
			workspaceID, messageID, cursorOccurredAt, cursorID, limit+1,
		)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var events []domain.TrackingEvent
	for rows.Next() {
		var evt domain.TrackingEvent
		var metadataJSON []byte
		var trackingLinkID, provider, providerEventID, providerMessageID *string

		if err := rows.Scan(&evt.ID, &evt.WorkspaceID, &evt.MessageID, &trackingLinkID,
			&evt.EventType, &evt.Source, &evt.SourceEventID,
			&provider, &providerEventID, &providerMessageID,
			&evt.OccurredAt, &evt.ReceivedAt, &metadataJSON, &evt.CreatedAt); err != nil {
			return nil, "", err
		}

		if trackingLinkID != nil {
			evt.TrackingLinkID = *trackingLinkID
		}
		if provider != nil {
			evt.Provider = *provider
		}
		if providerEventID != nil {
			evt.ProviderEventID = *providerEventID
		}
		if providerMessageID != nil {
			evt.ProviderMessageID = *providerMessageID
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &evt.MetadataJSON); err != nil {
				evt.MetadataJSON = map[string]any{}
			}
		} else {
			evt.MetadataJSON = map[string]any{}
		}

		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(events) > limit {
		events = events[:limit]
		last := events[len(events)-1]
		nextCursor = formatCursor(last.OccurredAt, last.ID)
	}

	if events == nil {
		events = []domain.TrackingEvent{}
	}
	return events, nextCursor, nil
}

func packJSON(v map[string]any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseCursor(cursor string, occurredAt *time.Time, id *string) error {
	parts := strings.SplitN(cursor, "_", 2)
	if len(parts) != 2 {
		return errors.New("invalid cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return err
	}
	*occurredAt = t
	*id = parts[1]
	return nil
}

func formatCursor(t time.Time, id string) string {
	return t.UTC().Format(time.RFC3339Nano) + "_" + id
}

func (r *TrackingLinkRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.db
}

func (r *TrackingEventRepository) getDB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.db
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
	_, err := db.Exec(ctx,
		`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, workspace_id, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload,
		event.WorkspaceID, event.OccurredAt,
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
