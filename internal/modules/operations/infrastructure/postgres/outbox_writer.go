package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type OutboxWriterRepo struct {
	db    platformpostgres.DBTX
	idGen func() (string, error)
}

func NewOutboxWriterRepo(db platformpostgres.DBTX) *OutboxWriterRepo {
	return &OutboxWriterRepo{
		db:    db,
		idGen: id.NewUUIDGenerator().New,
	}
}

func (w *OutboxWriterRepo) Write(ctx context.Context, eventType, aggregateID, workspaceID string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	eventID, err := w.idGen()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = w.db.Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers, workspace_id, occurred_at, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, '{}'::jsonb, $6, $7, $8)`,
		eventID, "replay_job", aggregateID, eventType, payloadJSON, workspaceID, now, now,
	)
	return err
}
