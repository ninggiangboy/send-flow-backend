package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type CampaignRepository struct {
	conn driver.Conn
}

func NewCampaignRepository(conn driver.Conn) *CampaignRepository {
	return &CampaignRepository{conn: conn}
}

func (r *CampaignRepository) GetCampaignFunnel(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignFunnel, error) {
	query := `
		SELECT
			workspace_id,
			campaign_id,
			countIf(event_type = 'queued') AS queued_count,
			countIf(event_type = 'accepted') AS accepted_count,
			countIf(event_type = 'delivered') AS delivered_count,
			countIf(event_type = 'bounced') AS bounced_count,
			countIf(event_type = 'complained') AS complained_count,
			countIf(event_type = 'opened') AS opened_count,
			countIf(event_type = 'clicked') AS clicked_count,
			countIf(event_type = 'unsubscribed') AS unsubscribed_count,
			countIf(event_type = 'retry_scheduled') AS retry_scheduled_count,
			max(occurred_at) AS last_event_at
		FROM email_events FINAL
		WHERE workspace_id = ? AND campaign_id = ?
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
	query += " GROUP BY workspace_id, campaign_id"

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	f := domain.CampaignFunnel{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
	}

	if !rows.Next() {
		f.Status = "pending"
		return &f, nil
	}

	var lastEventAt *time.Time
	if err := rows.Scan(
		&f.WorkspaceID, &f.CampaignID,
		&f.QueuedCount, &f.AcceptedCount, &f.DeliveredCount,
		&f.BouncedCount, &f.ComplainedCount,
		&f.OpenedCount, &f.ClickedCount, &f.UnsubscribedCount,
		&f.RetryScheduledCount, &lastEventAt,
	); err != nil {
		return nil, err
	}

	delivered := f.DeliveredCount
	accepted := f.AcceptedCount
	f.DeliveryRate = domain.ComputeRate(delivered, accepted)
	f.BounceRate = domain.ComputeRate(f.BouncedCount, delivered)
	f.ComplaintRate = domain.ComputeRate(f.ComplainedCount, delivered)
	f.OpenRate = domain.ComputeRate(f.OpenedCount, delivered)
	f.ClickRate = domain.ComputeRate(f.ClickedCount, delivered)
	f.UnsubscribeRate = domain.ComputeRate(f.UnsubscribedCount, delivered)
	f.Status = "ready"
	if lastEventAt != nil {
		f.LastEventAt = lastEventAt.Format(time.RFC3339)
	}

	return &f, nil
}

func (r *CampaignRepository) GetCampaignTimeSeries(ctx context.Context, workspaceID, campaignID string, from, to time.Time, interval, eventType string) (*domain.CampaignTimeSeriesResult, error) {
	var intervalFunc string
	switch interval {
	case "hour":
		intervalFunc = "toStartOfHour(occurred_at)"
	case "day":
		intervalFunc = "toStartOfDay(occurred_at)"
	case "week":
		intervalFunc = "toStartOfWeek(occurred_at)"
	default:
		intervalFunc = "toStartOfDay(occurred_at)"
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS bucket_start,
			event_type,
			count() AS cnt
		FROM email_events FINAL
		WHERE workspace_id = ? AND campaign_id = ?
	`, intervalFunc)
	args := []any{workspaceID, campaignID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if eventType != "" {
		query += " AND event_type = ?"
		args = append(args, eventType)
	}

	query += fmt.Sprintf(` GROUP BY bucket_start, event_type ORDER BY bucket_start, event_type`)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []domain.CampaignTimeSeriesBucket
	for rows.Next() {
		var b domain.CampaignTimeSeriesBucket
		if err := rows.Scan(&b.BucketStart, &b.EventType, &b.Count); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}

	return &domain.CampaignTimeSeriesResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		Buckets:     buckets,
	}, nil
}

func (r *CampaignRepository) GetCampaignBreakdown(ctx context.Context, workspaceID, campaignID string, from, to time.Time, groupBy string) (*domain.CampaignBreakdownResult, error) {
	var groupField string
	switch groupBy {
	case "provider":
		groupField = "provider"
	case "recipient_domain":
		groupField = "recipient_domain"
	case "event_type":
		groupField = "event_type"
	default:
		return nil, fmt.Errorf("%w: unsupported group_by %q", domain.ErrAnalyticsQueryInvalid, groupBy)
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS group_key,
			event_type,
			count() AS cnt,
			max(occurred_at) AS last_event_at
		FROM email_events FINAL
		WHERE workspace_id = ? AND campaign_id = ?
	`, groupField)
	args := []any{workspaceID, campaignID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}

	query += fmt.Sprintf(` GROUP BY group_key, event_type ORDER BY group_key, event_type`)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rowsOut []domain.CampaignBreakdownRow
	for rows.Next() {
		var row domain.CampaignBreakdownRow
		var lastEventAt *time.Time
		if err := rows.Scan(&row.GroupKey, &row.EventType, &row.Count, &lastEventAt); err != nil {
			return nil, err
		}
		if lastEventAt != nil {
			row.LastEventAt = lastEventAt.Format(time.RFC3339)
		}
		rowsOut = append(rowsOut, row)
	}

	return &domain.CampaignBreakdownResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		GroupBy:     groupBy,
		Rows:        rowsOut,
	}, nil
}

func (r *CampaignRepository) GetCampaignEvents(ctx context.Context, f domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
	query := `
		SELECT
			source_event_id, source_event_type, message_id,
			provider, provider_message_id,
			event_type, recipient_domain,
			occurred_at, received_at
		FROM email_events FINAL
		WHERE workspace_id = ? AND campaign_id = ?
	`
	args := []any{f.WorkspaceID, f.CampaignID}

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

	if f.Cursor != "" {
		parts := strings.SplitN(f.Cursor, "|", 2)
		if len(parts) == 2 {
			query += " AND (occurred_at, source_event_id) < (?, ?)"
			args = append(args, parts[0], parts[1])
		} else {
			query += " AND occurred_at < ?"
			args = append(args, f.Cursor)
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

	var events []domain.CampaignEventRow
	hasMore := false
	for rows.Next() {
		if len(events) == limit {
			hasMore = true
			break
		}
		var e domain.CampaignEventRow
		var occurredAt, receivedAt time.Time
		if err := rows.Scan(
			&e.SourceEventID, &e.SourceEventType, &e.MessageID,
			&e.Provider, &e.ProviderMessageID,
			&e.EventType, &e.RecipientDomain,
			&occurredAt, &receivedAt,
		); err != nil {
			return nil, err
		}
		e.OccurredAt = occurredAt.Format(time.RFC3339)
		e.ReceivedAt = receivedAt.Format(time.RFC3339)
		events = append(events, e)
	}

	result := &domain.CampaignEventsResult{
		Status:      "ready",
		WorkspaceID: f.WorkspaceID,
		CampaignID:  f.CampaignID,
		Events:      events,
	}
	if hasMore && len(events) > 0 {
		last := events[len(events)-1]
		result.NextCursor = last.OccurredAt + "|" + last.SourceEventID
	}

	return result, nil
}
