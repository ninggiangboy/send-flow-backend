package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type OutboxRepository struct {
	db platformpostgres.DBTX
}

func NewOutboxRepository(db platformpostgres.DBTX) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Save(ctx context.Context, event ports.OutboxEvent) error {
	headersJSON, err := json.Marshal(event.Headers)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = r.db.Exec(ctx,
		`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers, workspace_id, occurred_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload, headersJSON, event.WorkspaceID, event.OccurredAt, now,
	)
	return err
}
