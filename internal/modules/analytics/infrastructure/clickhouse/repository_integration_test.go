//go:build integration

package clickhouse

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"
)

var (
	globalConn      driver.Conn
	globalContainer *tcclickhouse.ClickHouseContainer
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	ctx := context.Background()

	container, err := tcclickhouse.Run(ctx,
		"clickhouse/clickhouse-server:24.8",
		tcclickhouse.WithUsername("default"),
		tcclickhouse.WithPassword("testpass"),
	)
	if err != nil {
		panic("start clickhouse container: " + err.Error())
	}
	globalContainer = container

	dsn, err := container.ConnectionString(ctx)
	if err != nil {
		container.Terminate(ctx)
		panic("build dsn: " + err.Error())
	}

	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		container.Terminate(ctx)
		panic("parse dsn: " + err.Error())
	}
	opts.Settings = clickhouse.Settings{
		"max_execution_time": 60,
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		container.Terminate(ctx)
		panic("open clickhouse: " + err.Error())
	}

	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		container.Terminate(ctx)
		panic("ping clickhouse: " + err.Error())
	}

	schema := []string{
		`CREATE TABLE IF NOT EXISTS email_events (
			source_event_id String,
			source_event_type String,
			workspace_id String,
			campaign_id String,
			message_id String,
			provider String,
			provider_message_id String,
			provider_event_id String,
			event_type String,
			recipient_domain String,
			occurred_at DateTime64(3),
			received_at DateTime64(3),
			metadata_json String,
			created_at DateTime64(3)
		) ENGINE = ReplacingMergeTree()
		ORDER BY (workspace_id, campaign_id, occurred_at, source_event_id)`,
		`CREATE TABLE IF NOT EXISTS operations_events (
			source_event_id String,
			source String,
			source_event_type String,
			operation_type String,
			status String,
			workspace_id String,
			error_type String,
			consumer String,
			target String,
			metadata_json String,
			occurred_at DateTime64(3),
			created_at DateTime64(3)
		) ENGINE = ReplacingMergeTree()
		ORDER BY (workspace_id, source, occurred_at, source_event_id)`,
	}
	for _, stmt := range schema {
		if err := conn.Exec(ctx, stmt); err != nil {
			conn.Close()
			container.Terminate(ctx)
			panic("create table: " + err.Error() + "\nSQL: " + stmt)
		}
	}

	globalConn = conn
}

func teardown() {
	if globalConn != nil {
		globalConn.Close()
	}
	if globalContainer != nil {
		globalContainer.Terminate(context.Background())
	}
}

func chConn() driver.Conn {
	return globalConn
}

func cleanTables(ctx context.Context, t *testing.T, conn driver.Conn) {
	t.Helper()
	for _, table := range []string{"email_events", "operations_events"} {
		if err := conn.Exec(ctx, "TRUNCATE TABLE "+table); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}

func insertEmailEvent(ctx context.Context, t *testing.T, conn driver.Conn, e domain.EmailEventFact) {
	t.Helper()
	err := conn.Exec(ctx,
		`INSERT INTO email_events (
			source_event_id, source_event_type, workspace_id,
			campaign_id, message_id, provider,
			provider_message_id, provider_event_id, event_type,
			recipient_domain, occurred_at, received_at,
			metadata_json, created_at
		) VALUES (
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			'{}', now()
		)`,
		e.SourceEventID, e.SourceEventType, e.WorkspaceID,
		e.CampaignID, e.MessageID, e.Provider,
		e.ProviderMessageID, e.ProviderEventID, e.EventType,
		e.RecipientDomain, e.OccurredAt, e.ReceivedAt,
	)
	if err != nil {
		t.Fatalf("insert email event: %v", err)
	}
}

func insertOperationEvent(
	ctx context.Context, t *testing.T, conn driver.Conn,
	sourceEventID, source, sourceEventType, operationType, status, workspaceID,
	errorType, consumer, target string, occurredAt time.Time,
) {
	t.Helper()
	err := conn.Exec(ctx,
		`INSERT INTO operations_events (
			source_event_id, source, source_event_type,
			operation_type, status, workspace_id,
			error_type, consumer, target,
			metadata_json, occurred_at, created_at
		) VALUES (
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			'{}', ?, now()
		)`,
		sourceEventID, source, sourceEventType,
		operationType, status, workspaceID,
		errorType, consumer, target,
		occurredAt,
	)
	if err != nil {
		t.Fatalf("insert operation event: %v", err)
	}
}

func TestForensicsRepositoryIntegration_SearchEvents(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewForensicsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertEmailEvent(ctx, t, conn, domain.EmailEventFact{
		SourceEventID: "src-1", SourceEventType: "test.event",
		WorkspaceID: "ws-1", CampaignID: "camp-1", MessageID: "msg-1",
		Provider: "sendgrid", EventType: domain.EventTypeDelivered,
		OccurredAt: now, ReceivedAt: now,
	})

	result, err := repo.SearchEvents(ctx, domain.ForensicQueryFilter{
		WorkspaceID: "ws-1",
		From:        now.Add(-time.Hour),
		To:          now.Add(time.Hour),
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].SourceEventID != "src-1" {
		t.Fatalf("expected src-1, got %s", result.Events[0].SourceEventID)
	}
	if result.Events[0].EventType != "delivered" {
		t.Fatalf("expected delivered, got %s", result.Events[0].EventType)
	}
}

func TestForensicsRepositoryIntegration_GetMessageTimeline(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewForensicsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertEmailEvent(ctx, t, conn, domain.EmailEventFact{
		SourceEventID: "src-1", SourceEventType: "test.event",
		WorkspaceID: "ws-1", MessageID: "msg-1",
		Provider: "sendgrid", EventType: domain.EventTypeAccepted,
		OccurredAt: now, ReceivedAt: now,
	})
	insertEmailEvent(ctx, t, conn, domain.EmailEventFact{
		SourceEventID: "src-2", SourceEventType: "test.event",
		WorkspaceID: "ws-1", MessageID: "msg-1",
		Provider: "sendgrid", EventType: domain.EventTypeDelivered,
		OccurredAt: now.Add(time.Minute), ReceivedAt: now.Add(time.Minute),
	})

	result, err := repo.GetMessageTimeline(ctx, "ws-1", "msg-1")
	if err != nil {
		t.Fatalf("GetMessageTimeline: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
	if result.Events[0].EventType != "accepted" {
		t.Fatalf("expected accepted first, got %s", result.Events[0].EventType)
	}
	if result.Events[1].EventType != "delivered" {
		t.Fatalf("expected delivered second, got %s", result.Events[1].EventType)
	}
}

func TestForensicsRepositoryIntegration_GetProviderEventTrace(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewForensicsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertEmailEvent(ctx, t, conn, domain.EmailEventFact{
		SourceEventID: "src-1", SourceEventType: "test.event",
		WorkspaceID: "ws-1", CampaignID: "camp-1", MessageID: "msg-1",
		Provider: "sendgrid", ProviderMessageID: "sg-msg-1", ProviderEventID: "pe-1",
		EventType:  domain.EventTypeDelivered,
		OccurredAt: now, ReceivedAt: now,
	})

	result, err := repo.GetProviderEventTrace(ctx, "ws-1", "pe-1")
	if err != nil {
		t.Fatalf("GetProviderEventTrace: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if result.CampaignID != "camp-1" {
		t.Fatalf("expected camp-1, got %s", result.CampaignID)
	}
	if result.MessageID != "msg-1" {
		t.Fatalf("expected msg-1, got %s", result.MessageID)
	}
}

func TestForensicsRepositoryIntegration_GetCampaignIncidentTimeline(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewForensicsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertEmailEvent(ctx, t, conn, domain.EmailEventFact{
		SourceEventID: "src-1", SourceEventType: "test.event",
		WorkspaceID: "ws-1", CampaignID: "camp-1", MessageID: "msg-1",
		Provider: "sendgrid", EventType: domain.EventTypeBounced,
		OccurredAt: now, ReceivedAt: now,
	})
	insertEmailEvent(ctx, t, conn, domain.EmailEventFact{
		SourceEventID: "src-2", SourceEventType: "test.event",
		WorkspaceID: "ws-1", CampaignID: "camp-1", MessageID: "msg-2",
		Provider: "ses", EventType: domain.EventTypeComplained,
		OccurredAt: now.Add(time.Hour), ReceivedAt: now.Add(time.Hour),
	})

	result, err := repo.GetCampaignIncidentTimeline(ctx, "ws-1", "camp-1", now.Add(-time.Hour), now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("GetCampaignIncidentTimeline: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
}

func TestForensicsRepositoryIntegration_GetProviderEventTraceNotFound(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewForensicsRepository(conn)

	result, err := repo.GetProviderEventTrace(ctx, "ws-1", "nonexistent")
	if err != nil {
		t.Fatalf("GetProviderEventTrace: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil for not found")
	}
}

// --- Operations repository integration tests ---

func TestOperationsRepositoryIntegration_GetOutboxLag(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewOperationsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertOperationEvent(ctx, t, conn,
		"src-1", "delivery", "send_email", "outbox_lag", "", "ws-1",
		"", "", "", now,
	)
	insertOperationEvent(ctx, t, conn,
		"src-2", "delivery", "send_email", "outbox_lag", "", "ws-1",
		"", "", "", now.Add(time.Hour),
	)

	result, err := repo.GetOutboxLag(ctx, "ws-1", now.Add(-time.Hour), now.Add(2*time.Hour), "")
	if err != nil {
		t.Fatalf("GetOutboxLag: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if result.Rows[0].Source != "delivery" {
		t.Fatalf("expected delivery, got %s", result.Rows[0].Source)
	}
}

func TestOperationsRepositoryIntegration_GetConsumerFailures(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewOperationsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertOperationEvent(ctx, t, conn,
		"src-1", "delivery", "analytics_event", "consumer_failure", "failed", "ws-1",
		"timeout", "analytics_events", "", now,
	)

	result, err := repo.GetConsumerFailures(ctx, "ws-1", now.Add(-time.Hour), now.Add(time.Hour), "")
	if err != nil {
		t.Fatalf("GetConsumerFailures: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Consumer != "analytics_events" {
		t.Fatalf("expected analytics_events, got %s", result.Rows[0].Consumer)
	}
}

func TestOperationsRepositoryIntegration_GetDLQVolume(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewOperationsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertOperationEvent(ctx, t, conn,
		"src-1", "delivery", "send_email", "dlq_created", "", "ws-1",
		"", "", "", now,
	)

	result, err := repo.GetDLQVolume(ctx, "ws-1", now.Add(-time.Hour), now.Add(time.Hour), "")
	if err != nil {
		t.Fatalf("GetDLQVolume: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestOperationsRepositoryIntegration_GetWebhookDeliveryTimeSeries(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewOperationsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertOperationEvent(ctx, t, conn,
		"src-1", "webhooks", "webhook.delivery", "webhook_succeeded", "success", "ws-1",
		"", "", "https://example.com/hook", now,
	)

	result, err := repo.GetWebhookDeliveryTimeSeries(ctx, "ws-1", now.Add(-time.Hour), now.Add(time.Hour), "", "day")
	if err != nil {
		t.Fatalf("GetWebhookDeliveryTimeSeries: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets))
	}
}

func TestOperationsRepositoryIntegration_GetWebhookReliability(t *testing.T) {
	conn := chConn()
	ctx := context.Background()
	cleanTables(ctx, t, conn)
	repo := NewOperationsRepository(conn)

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	insertOperationEvent(ctx, t, conn,
		"src-1", "webhooks", "webhook.delivery", "webhook_succeeded", "success", "ws-1",
		"", "", "https://example.com/hook", now,
	)
	insertOperationEvent(ctx, t, conn,
		"src-2", "webhooks", "webhook.delivery", "webhook_failed", "failure", "ws-1",
		"", "", "https://example.com/hook", now,
	)

	result, err := repo.GetWebhookReliability(ctx, "ws-1", now.Add(-time.Hour), now.Add(time.Hour), "")
	if err != nil {
		t.Fatalf("GetWebhookReliability: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].TotalCount != 2 {
		t.Fatalf("expected 2 total, got %d", result.Rows[0].TotalCount)
	}
	if result.Rows[0].Succeeded != 1 {
		t.Fatalf("expected 1 succeeded, got %d", result.Rows[0].Succeeded)
	}
	if result.Rows[0].Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Rows[0].Failed)
	}
}
