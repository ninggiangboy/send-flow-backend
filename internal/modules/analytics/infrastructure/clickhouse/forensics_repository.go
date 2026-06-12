package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type ForensicsRepository struct {
	conn driver.Conn
}

func NewForensicsRepository(conn driver.Conn) *ForensicsRepository {
	return &ForensicsRepository{conn: conn}
}

func (r *ForensicsRepository) SearchEvents(ctx context.Context, f domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error) {
	query := `
		SELECT
			source_event_id, source_event_type, workspace_id,
			campaign_id, message_id,
			provider, provider_message_id, provider_event_id,
			event_type, recipient_domain,
			occurred_at, received_at
		FROM email_events
		WHERE workspace_id = ?
	`
	args := []any{f.WorkspaceID}

	if !f.From.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, f.To)
	}
	if f.EventType != "" {
		query += " AND event_type = ?"
		args = append(args, f.EventType)
	}
	if f.Provider != "" {
		query += " AND provider = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(f.Provider)))
	}
	if f.Domain != "" {
		query += " AND recipient_domain = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(f.Domain)))
	}
	if f.CampaignID != "" {
		query += " AND campaign_id = ?"
		args = append(args, f.CampaignID)
	}
	if f.MessageID != "" {
		query += " AND message_id = ?"
		args = append(args, f.MessageID)
	}

	if f.Cursor != "" {
		parts := strings.SplitN(f.Cursor, "|", 2)
		if len(parts) == 2 {
			cursorTime, err := time.Parse(time.RFC3339, parts[0])
			if err != nil {
				return nil, fmt.Errorf("parse cursor timestamp: %w", err)
			}
			query += " AND (occurred_at, source_event_id) < (?, ?)"
			args = append(args, cursorTime, parts[1])
		} else {
			cursorTime, err := time.Parse(time.RFC3339, f.Cursor)
			if err != nil {
				return nil, fmt.Errorf("parse cursor timestamp: %w", err)
			}
			query += " AND occurred_at < ?"
			args = append(args, cursorTime)
		}
	}

	query += " ORDER BY occurred_at DESC, source_event_id DESC"

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT %d", limit+1)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.ForensicEventRow
	hasMore := false
	var lastOccurredAt time.Time
	for rows.Next() {
		if len(events) == limit {
			hasMore = true
			break
		}
		var e domain.ForensicEventRow
		var occurredAt, receivedAt time.Time
		if err := rows.Scan(
			&e.SourceEventID, &e.SourceEventType, &e.WorkspaceID,
			&e.CampaignID, &e.MessageID,
			&e.Provider, &e.ProviderMessageID, &e.ProviderEventID,
			&e.EventType, &e.RecipientDomain,
			&occurredAt, &receivedAt,
		); err != nil {
			return nil, err
		}
		e.OccurredAt = occurredAt.Format(time.RFC3339Nano)
		e.ReceivedAt = receivedAt.Format(time.RFC3339Nano)
		lastOccurredAt = occurredAt
		events = append(events, e)
	}

	result := &domain.ForensicEventsResult{
		Status:      "ready",
		WorkspaceID: f.WorkspaceID,
		Events:      events,
	}
	if hasMore && len(events) > 0 {
		last := events[len(events)-1]
		result.NextCursor = lastOccurredAt.Format(time.RFC3339Nano) + "|" + last.SourceEventID
	}

	return result, nil
}

func (r *ForensicsRepository) GetMessageTimeline(ctx context.Context, workspaceID, messageID string) (*domain.MessageTimelineResult, error) {
	query := `
		SELECT
			source_event_id, source_event_type,
			event_type,
			provider, provider_message_id, provider_event_id,
			campaign_id, recipient_domain,
			occurred_at, received_at
		FROM email_events
		WHERE workspace_id = ? AND message_id = ?
		ORDER BY occurred_at ASC, source_event_id ASC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timeline []domain.MessageTimelineRow
	for rows.Next() {
		var e domain.MessageTimelineRow
		var occurredAt, receivedAt time.Time
		if err := rows.Scan(
			&e.SourceEventID, &e.SourceEventType,
			&e.EventType,
			&e.Provider, &e.ProviderMessageID, &e.ProviderEventID,
			&e.CampaignID, &e.RecipientDomain,
			&occurredAt, &receivedAt,
		); err != nil {
			return nil, err
		}
		e.OccurredAt = occurredAt.Format(time.RFC3339)
		e.ReceivedAt = receivedAt.Format(time.RFC3339)
		timeline = append(timeline, e)
	}

	return &domain.MessageTimelineResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		MessageID:   messageID,
		Events:      timeline,
	}, nil
}

func (r *ForensicsRepository) GetProviderEventTrace(ctx context.Context, workspaceID, providerEventID string) (*domain.ProviderEventTrace, error) {
	query := `
		SELECT
			workspace_id, campaign_id, message_id,
			provider, provider_message_id
		FROM email_events
		WHERE workspace_id = ? AND provider_event_id = ?
		LIMIT 1
	`
	rows, err := r.conn.Query(ctx, query, workspaceID, providerEventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var trace domain.ProviderEventTrace
	if err := rows.Scan(
		&trace.WorkspaceID, &trace.CampaignID, &trace.MessageID,
		&trace.Provider, &trace.ProviderMessageID,
	); err != nil {
		return nil, err
	}
	trace.Status = "ready"
	trace.ProviderEventID = providerEventID

	return &trace, nil
}

func (r *ForensicsRepository) GetCampaignIncidentTimeline(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignIncidentTimelineResult, error) {
	query := `
		SELECT
			source_event_id, source_event_type,
			event_type,
			message_id, provider, recipient_domain,
			occurred_at
		FROM email_events
		WHERE workspace_id = ? AND campaign_id = ?
			AND event_type IN ('bounced', 'complained')
	`
	args := []any{workspaceID, campaignID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}

	query += " ORDER BY occurred_at DESC, source_event_id DESC LIMIT 100"

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.CampaignIncidentTimelineRow
	for rows.Next() {
		var e domain.CampaignIncidentTimelineRow
		var occurredAt time.Time
		if err := rows.Scan(
			&e.SourceEventID, &e.SourceEventType,
			&e.EventType,
			&e.MessageID, &e.Provider, &e.RecipientDomain,
			&occurredAt,
		); err != nil {
			return nil, err
		}
		e.OccurredAt = occurredAt.Format(time.RFC3339)
		events = append(events, e)
	}

	return &domain.CampaignIncidentTimelineResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		Events:      events,
	}, nil
}
