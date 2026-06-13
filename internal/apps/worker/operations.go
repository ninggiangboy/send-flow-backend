package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type ProcessedEventMarkers struct {
	db execer
}

func NewProcessedEventMarkers(pool *pgxpool.Pool) *ProcessedEventMarkers {
	return &ProcessedEventMarkers{db: pool}
}

func NewProcessedEventMarkersWithExecer(db execer) *ProcessedEventMarkers {
	return &ProcessedEventMarkers{db: db}
}

func (m *ProcessedEventMarkers) WasProcessed(ctx context.Context, consumerName, eventID string) (bool, error) {
	var exists bool
	err := m.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM processed_event_markers WHERE consumer_name=$1 AND event_id=$2::uuid)", consumerName, eventID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (m *ProcessedEventMarkers) MarkProcessed(ctx context.Context, consumerName, eventID string) (bool, error) {
	_, err := m.db.Exec(ctx, "INSERT INTO processed_event_markers (consumer_name, event_id) VALUES ($1, $2::uuid)", consumerName, eventID)
	if err == nil {
		return true, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return false, nil
	}
	return false, err
}

type DeadLetterRepository struct {
	db execer
}

type DeadLetterRecord struct {
	ID              string
	WorkspaceID     string
	Source          string
	SourceEventType string
	EventID         string
	Payload         any
	ErrorMessage    string
	Retryable       bool
}

func NewDeadLetterRepository(pool *pgxpool.Pool) *DeadLetterRepository {
	return &DeadLetterRepository{db: pool}
}

func NewDeadLetterRepositoryWithExecer(db execer) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

func workspaceIDFromEventPayload(payload []byte) string {
	var env struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return ""
	}
	return env.WorkspaceID
}

func eventTypeFromEnvelope(payload []byte) string {
	var env struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return ""
	}
	return env.EventType
}

func workspaceIDFromMessage(headers map[string]string, payload []byte) string {
	if ws := headers["workspace_id"]; ws != "" {
		return ws
	}
	return workspaceIDFromEventPayload(payload)
}

func marshalPayload(v any) ([]byte, error) {
	switch p := v.(type) {
	case []byte:
		if json.Valid(p) {
			return p, nil
		}
		return json.Marshal(v)
	case json.RawMessage:
		if json.Valid(p) {
			return []byte(p), nil
		}
		return json.Marshal(v)
	default:
		return json.Marshal(v)
	}
}

func (r *DeadLetterRepository) Save(ctx context.Context, record DeadLetterRecord) error {
	payload, err := marshalPayload(record.Payload)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		ctx,
		"INSERT INTO dead_letter_records (id, workspace_id, source, source_event_type, event_id, payload, error_message, retryable) VALUES ($1, NULLIF($2, ''), $3, $8, NULLIF($4, '')::uuid, $5::jsonb, $6, $7)",
		record.ID,
		record.WorkspaceID,
		record.Source,
		record.EventID,
		payload,
		record.ErrorMessage,
		record.Retryable,
		record.SourceEventType,
	)
	return err
}

type OperationsEventRecorder func(ctx context.Context, source, sourceEventType, operationType, status, workspaceID, errorType, consumer, target string, occurredAt time.Time)

func newOpsRecorderAdapter(svc interface {
	IngestOperationsEvent(ctx context.Context, input analyticsapp.IngestOperationsEventInput) error
}) OperationsEventRecorder {
	return func(ctx context.Context, source, sourceEventType, operationType, status, workspaceID, errorType, consumer, target string, occurredAt time.Time) {
		_ = svc.IngestOperationsEvent(ctx, analyticsapp.IngestOperationsEventInput{
			Source:          source,
			SourceEventType: sourceEventType,
			OperationType:   operationType,
			Status:          status,
			WorkspaceID:     workspaceID,
			ErrorType:       errorType,
			Consumer:        consumer,
			Target:          target,
			OccurredAt:      occurredAt,
		})
	}
}
