package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type OperationsRepository struct {
	conn driver.Conn
}

func NewOperationsRepository(conn driver.Conn) *OperationsRepository {
	return &OperationsRepository{conn: conn}
}

func (r *OperationsRepository) GetOutboxLag(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.OutboxLagResult, error) {
	query := fmt.Sprintf(`
		SELECT
			toStartOfHour(occurred_at) AS bucket_start,
			source,
			source_event_type AS event_type,
			toInt64(count()) AS cnt,
			avg(dateDiff('second', occurred_at, created_at)) AS avg_lag
		FROM operations_events FINAL
		WHERE workspace_id = ?
			AND operation_type = '%s'
	`, contracts.OperationTypeOutboxLag)
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if source != "" {
		query += " AND source = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(source)))
	}

	query += ` GROUP BY bucket_start, source, event_type ORDER BY bucket_start, source, event_type`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OutboxLagRow
	for rows.Next() {
		var row domain.OutboxLagRow
		var bucketStart time.Time
		if err := rows.Scan(&bucketStart, &row.Source, &row.EventType, &row.Count, &row.LagSeconds); err != nil {
			return nil, err
		}
		row.BucketStart = bucketStart.Format(time.RFC3339)
		out = append(out, row)
	}

	return &domain.OutboxLagResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Rows:        out,
	}, nil
}

func (r *OperationsRepository) GetConsumerFailures(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.ConsumerFailureResult, error) {
	query := fmt.Sprintf(`
		SELECT
			toStartOfHour(occurred_at) AS bucket_start,
			source,
			consumer,
			error_type,
			toInt64(count()) AS cnt
		FROM operations_events FINAL
		WHERE workspace_id = ?
			AND operation_type = '%s'
	`, contracts.OperationTypeConsumerFailure)
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if source != "" {
		query += " AND source = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(source)))
	}

	query += ` GROUP BY bucket_start, source, consumer, error_type ORDER BY bucket_start, source, consumer, error_type`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ConsumerFailureRow
	for rows.Next() {
		var row domain.ConsumerFailureRow
		var bucketStart time.Time
		if err := rows.Scan(&bucketStart, &row.Source, &row.Consumer, &row.ErrorType, &row.Count); err != nil {
			return nil, err
		}
		row.BucketStart = bucketStart.Format(time.RFC3339)
		out = append(out, row)
	}

	return &domain.ConsumerFailureResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Rows:        out,
	}, nil
}

func (r *OperationsRepository) GetDLQVolume(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.DLQResult, error) {
	query := fmt.Sprintf(`
		SELECT
			toStartOfDay(occurred_at) AS bucket_start,
			source,
			source_event_type AS event_type,
			toInt64(count()) AS cnt
		FROM operations_events FINAL
		WHERE workspace_id = ?
			AND operation_type = '%s'
	`, contracts.OperationTypeDlqCreated)
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if source != "" {
		query += " AND source = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(source)))
	}

	query += ` GROUP BY bucket_start, source, event_type ORDER BY bucket_start, source, event_type`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DLQRow
	for rows.Next() {
		var row domain.DLQRow
		var bucketStart time.Time
		if err := rows.Scan(&bucketStart, &row.Source, &row.EventType, &row.Count); err != nil {
			return nil, err
		}
		row.BucketStart = bucketStart.Format(time.RFC3339)
		out = append(out, row)
	}

	return &domain.DLQResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Rows:        out,
	}, nil
}

func (r *OperationsRepository) GetWebhookDeliveryTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, status string, interval string) (*domain.WebhookDeliveryTimeSeriesResult, error) {
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
			status,
			toInt64(count()) AS cnt
		FROM operations_events FINAL
		WHERE workspace_id = ?
			AND source = 'webhooks'
			AND operation_type IN ('webhook_succeeded', 'webhook_failed', 'webhook_retry_scheduled')
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
	if status != "" {
		query += " AND status = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(status)))
	}

	query += ` GROUP BY bucket_start, status ORDER BY bucket_start, status`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []domain.WebhookDeliveryTimeSeriesBucket
	for rows.Next() {
		var b domain.WebhookDeliveryTimeSeriesBucket
		var bucketStart time.Time
		if err := rows.Scan(&bucketStart, &b.Status, &b.Count); err != nil {
			return nil, err
		}
		b.BucketStart = bucketStart.Format(time.RFC3339)
		buckets = append(buckets, b)
	}

	return &domain.WebhookDeliveryTimeSeriesResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Buckets:     buckets,
	}, nil
}

func (r *OperationsRepository) GetWebhookReliability(ctx context.Context, workspaceID string, from, to time.Time, target string) (*domain.WebhookReliabilityResult, error) {
	query := fmt.Sprintf(`
		SELECT
			source,
			target,
			toInt64(count()) AS total,
			toInt64(countIf(status = 'success')) AS succeeded,
			toInt64(countIf(status = 'failure')) AS failed,
			toInt64(countIf(status = 'retry')) AS retried
		FROM operations_events FINAL
		WHERE workspace_id = ?
			AND source = 'webhooks'
			AND operation_type IN ('%s', '%s', '%s')
	`, contracts.OperationTypeWebhookSucceeded, contracts.OperationTypeWebhookFailed, contracts.OperationTypeWebhookRetryScheduled)
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}
	if target != "" {
		query += " AND target = ?"
		args = append(args, strings.TrimSpace(target))
	}

	query += ` GROUP BY source, target ORDER BY source, target`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WebhookReliabilityRow
	for rows.Next() {
		var row domain.WebhookReliabilityRow
		if err := rows.Scan(&row.Source, &row.Target, &row.TotalCount, &row.Succeeded, &row.Failed, &row.Retried); err != nil {
			return nil, err
		}
		row.SuccessRate = domain.ComputeRate(row.Succeeded, row.TotalCount)
		out = append(out, row)
	}

	return &domain.WebhookReliabilityResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Rows:        out,
	}, nil
}
