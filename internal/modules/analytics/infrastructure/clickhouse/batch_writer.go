package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type BatchWriter struct {
	conn         driver.Conn
	dedupeWindow time.Duration
}

type BatchWriterOption func(*BatchWriter)

func WithDedupeWindow(d time.Duration) BatchWriterOption {
	return func(w *BatchWriter) {
		w.dedupeWindow = d
	}
}

func NewBatchWriter(conn driver.Conn, opts ...BatchWriterOption) *BatchWriter {
	w := &BatchWriter{conn: conn}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *BatchWriter) CreateBatch(ctx context.Context, facts []domain.EmailEventFact) error {
	toInsert := facts

	if w.dedupeWindow > 0 {
		filtered, err := w.filterExisting(ctx, facts)
		if err != nil {
			return err
		}
		if len(filtered) == 0 {
			return nil
		}
		toInsert = filtered
	}

	batch, err := w.conn.PrepareBatch(ctx, `
		INSERT INTO email_events (
			source_event_id, source_event_type, workspace_id, campaign_id, message_id,
			provider, provider_message_id, provider_event_id, event_type, recipient_domain,
			occurred_at, received_at, metadata_json, created_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?
		)
	`)
	if err != nil {
		return err
	}

	for _, f := range toInsert {
		mdStr := "{}"
		if f.Metadata != nil {
			mdBytes, err := json.Marshal(f.Metadata)
			if err != nil {
				return err
			}
			mdStr = string(mdBytes)
		}

		if err := batch.Append(
			f.SourceEventID, f.SourceEventType, f.WorkspaceID,
			f.CampaignID, f.MessageID,
			f.Provider, f.ProviderMessageID, f.ProviderEventID,
			f.EventType, f.RecipientDomain,
			f.OccurredAt, f.ReceivedAt, mdStr, f.CreatedAt,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (w *BatchWriter) filterExisting(ctx context.Context, facts []domain.EmailEventFact) ([]domain.EmailEventFact, error) {
	ids := make([]string, 0, len(facts))
	idSet := make(map[string]int, len(facts))
	for i, f := range facts {
		if _, ok := idSet[f.SourceEventID]; !ok {
			idSet[f.SourceEventID] = i
			ids = append(ids, f.SourceEventID)
		}
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT DISTINCT source_event_id FROM email_events WHERE source_event_id IN (%s) AND created_at >= now() - INTERVAL %d SECOND",
		strings.Join(placeholders, ","),
		int(w.dedupeWindow.Seconds()),
	)
	rows, err := w.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existing := make(map[string]bool, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		existing[id] = true
	}

	if len(existing) == 0 {
		return facts, nil
	}

	filtered := make([]domain.EmailEventFact, 0, len(facts))
	for _, f := range facts {
		if !existing[f.SourceEventID] {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}
