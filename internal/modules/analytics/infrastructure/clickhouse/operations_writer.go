package clickhouse

import (
	"context"
	"encoding/json"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type OperationsWriter struct {
	conn driver.Conn
}

func NewOperationsWriter(conn driver.Conn) *OperationsWriter {
	return &OperationsWriter{conn: conn}
}

func (w *OperationsWriter) Create(ctx context.Context, event ports.OperationsEvent) error {
	md, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}

	sourceEventID := event.SourceEventID
	if sourceEventID == "" {
		sourceEventID = event.Source + "_" + event.OperationType + "_" + event.OccurredAt.String()
	}

	return w.conn.Exec(ctx,
		`INSERT INTO operations_events (
			source_event_id, source, source_event_type, operation_type, status,
			workspace_id, error_type, consumer, target, metadata_json,
			occurred_at, created_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, now64(3)
		)`,
		sourceEventID, event.Source, event.SourceEventType, event.OperationType, event.Status,
		event.WorkspaceID, event.ErrorType, event.Consumer, event.Target, string(md),
		event.OccurredAt,
	)
}
