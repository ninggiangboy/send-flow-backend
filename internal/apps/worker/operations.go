package worker

import (
	"context"
	"encoding/json"
	"errors"

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
	ID           string
	Source       string
	EventID      string
	Payload      any
	ErrorMessage string
	Retryable    bool
}

func NewDeadLetterRepository(pool *pgxpool.Pool) *DeadLetterRepository {
	return &DeadLetterRepository{db: pool}
}

func NewDeadLetterRepositoryWithExecer(db execer) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

func (r *DeadLetterRepository) Save(ctx context.Context, record DeadLetterRecord) error {
	payload, err := json.Marshal(record.Payload)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		ctx,
		"INSERT INTO dead_letter_records (id, source, event_id, payload, error_message, retryable) VALUES ($1, $2, NULLIF($3, '')::uuid, $4::jsonb, $5, $6)",
		record.ID,
		record.Source,
		record.EventID,
		payload,
		record.ErrorMessage,
		record.Retryable,
	)
	return err
}
