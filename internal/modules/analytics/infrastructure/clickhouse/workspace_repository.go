package clickhouse

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type WorkspaceRepository struct {
	conn driver.Conn
}

func NewWorkspaceRepository(conn driver.Conn) *WorkspaceRepository {
	return &WorkspaceRepository{conn: conn}
}

func (r *WorkspaceRepository) GetWorkspaceOverview(ctx context.Context, workspaceID string) (*domain.WorkspaceAnalyticsOverview, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			workspace_id,
			countIf(event_type = 'queued')          AS queued_count,
			countIf(event_type = 'accepted')         AS accepted_count,
			countIf(event_type = 'delivered')        AS delivered_count,
			countIf(event_type = 'bounced')          AS bounced_count,
			countIf(event_type = 'complained')       AS complained_count,
			countIf(event_type = 'opened')           AS opened_count,
			countIf(event_type = 'clicked')          AS clicked_count,
			countIf(event_type = 'unsubscribed')     AS unsubscribed_count,
			countIf(event_type = 'retry_scheduled')  AS retry_scheduled_count,
			max(occurred_at)                         AS last_event_at
		FROM email_events FINAL
		WHERE workspace_id = ?
		GROUP BY workspace_id`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, domain.ErrAnalyticsProjectionNotFound
	}

	var overview domain.WorkspaceAnalyticsOverview
	var lastEventAt *time.Time
	if err := rows.Scan(
		&overview.WorkspaceID,
		&overview.QueuedCount, &overview.AcceptedCount, &overview.DeliveredCount,
		&overview.BouncedCount, &overview.ComplainedCount,
		&overview.OpenedCount, &overview.ClickedCount,
		&overview.UnsubscribedCount, &overview.RetryScheduledCount,
		&lastEventAt,
	); err != nil {
		return nil, err
	}
	overview.LastEventAt = lastEventAt
	if lastEventAt != nil {
		overview.LastUpdatedAt = *lastEventAt
	}
	return &overview, nil
}
