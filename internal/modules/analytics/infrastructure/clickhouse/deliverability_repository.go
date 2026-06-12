package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type DeliverabilityRepository struct {
	conn driver.Conn
}

func NewDeliverabilityRepository(conn driver.Conn) *DeliverabilityRepository {
	return &DeliverabilityRepository{conn: conn}
}

func (r *DeliverabilityRepository) GetDeliverabilityTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain, interval string) (*domain.DeliverabilityTimeSeriesResult, error) {
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
			provider,
			recipient_domain,
			event_type,
			count() AS cnt
		FROM email_events
		WHERE workspace_id = ?
	`, intervalFunc)
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if provider != "" {
		query += " AND provider = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(provider)))
	}
	if recipientDomain != "" {
		query += " AND recipient_domain = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(recipientDomain)))
	}

	query += fmt.Sprintf(` GROUP BY bucket_start, provider, recipient_domain, event_type ORDER BY bucket_start, provider, recipient_domain, event_type`)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []domain.DeliverabilityTimeSeriesBucket
	for rows.Next() {
		var b domain.DeliverabilityTimeSeriesBucket
		if err := rows.Scan(&b.BucketStart, &b.Provider, &b.RecipientDomain, &b.EventType, &b.Count); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}

	return &domain.DeliverabilityTimeSeriesResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Buckets:     buckets,
	}, nil
}

func (r *DeliverabilityRepository) GetDeliverabilityBreakdown(ctx context.Context, workspaceID string, from, to time.Time, groupBy, provider, recipientDomain string) (*domain.DeliverabilityBreakdownResult, error) {
	var groupField string
	switch groupBy {
	case "provider":
		groupField = "provider"
	case "recipient_domain":
		groupField = "recipient_domain"
	default:
		return nil, fmt.Errorf("%w: unsupported group_by %q for deliverability", domain.ErrAnalyticsQueryInvalid, groupBy)
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS group_key,
			event_type,
			count() AS cnt,
			max(occurred_at) AS last_event_at
		FROM email_events
		WHERE workspace_id = ?
	`, groupField)
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if provider != "" {
		query += " AND provider = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(provider)))
	}
	if recipientDomain != "" {
		query += " AND recipient_domain = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(recipientDomain)))
	}

	query += fmt.Sprintf(` GROUP BY group_key, event_type ORDER BY group_key, event_type`)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rowsOut []domain.DeliverabilityBreakdownRow
	totalByGroup := make(map[string]int64)
	for rows.Next() {
		var row domain.DeliverabilityBreakdownRow
		var lastEventAt *time.Time
		if err := rows.Scan(&row.GroupKey, &row.EventType, &row.Count, &lastEventAt); err != nil {
			return nil, err
		}
		if lastEventAt != nil {
			row.LastEventAt = lastEventAt.Format(time.RFC3339)
		}
		if groupBy == "provider" {
			row.Provider = row.GroupKey
		} else {
			row.RecipientDomain = row.GroupKey
		}
		if row.EventType == "delivered" {
			totalByGroup[row.GroupKey] = row.Count
		}
		rowsOut = append(rowsOut, row)
	}

	for i := range rowsOut {
		if denom, ok := totalByGroup[rowsOut[i].GroupKey]; ok && denom > 0 {
			rowsOut[i].Rate = domain.ComputeRate(rowsOut[i].Count, denom)
		}
	}

	return &domain.DeliverabilityBreakdownResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		GroupBy:     groupBy,
		Rows:        rowsOut,
	}, nil
}

func (r *DeliverabilityRepository) GetDeliverabilityLatency(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityLatencyResult, error) {
	query := `
		SELECT
			provider,
			recipient_domain,
			'accepted_to_delivered' AS event_type,
			count() AS cnt,
			quantile(0.50)(latency_ms) AS p50,
			quantile(0.95)(latency_ms) AS p95,
			quantile(0.99)(latency_ms) AS p99,
			avg(latency_ms) AS avg_latency
		FROM (
			SELECT
				a.provider,
				a.recipient_domain,
				dateDiff('millisecond', a.occurred_at, d.occurred_at) AS latency_ms
			FROM email_events a
			INNER JOIN email_events d
				ON a.workspace_id = d.workspace_id
				AND a.message_id = d.message_id
				AND a.campaign_id = d.campaign_id
			WHERE a.workspace_id = ?
				AND a.event_type = 'accepted'
				AND d.event_type = 'delivered'
	`
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND a.occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND a.occurred_at <= ?"
		args = append(args, to)
	}
	if provider != "" {
		query += " AND a.provider = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(provider)))
	}
	if recipientDomain != "" {
		query += " AND a.recipient_domain = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(recipientDomain)))
	}

	query += `)
		GROUP BY provider, recipient_domain
		ORDER BY provider, recipient_domain`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DeliverabilityLatencyRow
	for rows.Next() {
		var row domain.DeliverabilityLatencyRow
		if err := rows.Scan(&row.Provider, &row.RecipientDomain, &row.EventType, &row.Count, &row.P50LatencyMs, &row.P95LatencyMs, &row.P99LatencyMs, &row.AvgLatencyMs); err != nil {
			return nil, err
		}
		out = append(out, row)
	}

	return &domain.DeliverabilityLatencyResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Rows:        out,
	}, nil
}

func (r *DeliverabilityRepository) GetDeliverabilityIncidents(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityIncidentResult, error) {
	query := `
		SELECT
			provider,
			recipient_domain,
			'bounce' AS event_type,
			toStartOfDay(occurred_at) AS incident_date,
			count() AS cnt,
			countIf(event_type = 'bounced') AS bounce_count,
			countIf(event_type = 'delivered') AS delivered_count
		FROM email_events
		WHERE workspace_id = ?
			AND event_type IN ('bounced', 'delivered')
	`
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if provider != "" {
		query += " AND provider = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(provider)))
	}
	if recipientDomain != "" {
		query += " AND recipient_domain = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(recipientDomain)))
	}

	query += `
		GROUP BY provider, recipient_domain, incident_date
		HAVING (bounce_count * 1.0 / (bounce_count + delivered_count)) > 0.05
		ORDER BY incident_date DESC
		LIMIT 50`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DeliverabilityIncidentRow
	for rows.Next() {
		var row domain.DeliverabilityIncidentRow
		var incidentDate time.Time
		var totalCnt, bounceCnt, deliveredCnt int64
		if err := rows.Scan(&row.Provider, &row.RecipientDomain, &row.EventType, &incidentDate, &totalCnt, &bounceCnt, &deliveredCnt); err != nil {
			return nil, err
		}
		row.IncidentStart = incidentDate.Format(time.RFC3339)
		row.EventCount = bounceCnt
		row.Rate = domain.ComputeRate(bounceCnt, bounceCnt+deliveredCnt)
		out = append(out, row)
	}

	return &domain.DeliverabilityIncidentResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Provider:    provider,
		Domain:      recipientDomain,
		Rows:        out,
	}, nil
}
