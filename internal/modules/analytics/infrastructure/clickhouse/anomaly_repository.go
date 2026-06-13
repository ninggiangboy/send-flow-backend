package clickhouse

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type AnomalySignalRepository struct {
	conn driver.Conn
}

func NewAnomalySignalRepository(conn driver.Conn) *AnomalySignalRepository {
	return &AnomalySignalRepository{conn: conn}
}

func (r *AnomalySignalRepository) SaveAnomalySignals(ctx context.Context, workspaceID string, signals []domain.AnomalyRow) error {
	if len(signals) == 0 {
		return nil
	}

	batch, err := r.conn.PrepareBatch(ctx, `
		INSERT INTO anomaly_signals (
			anomaly_id, anomaly_type, severity, metric,
			observed, expected, deviation,
			workspace_id, window_start, window_end, detected_at
		) VALUES (
			?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?
		)
	`)
	if err != nil {
		return err
	}

	for _, s := range signals {
		windowStart, _ := time.Parse(time.RFC3339, s.WindowStart)
		windowEnd, _ := time.Parse(time.RFC3339, s.WindowEnd)
		detectedAt, _ := time.Parse(time.RFC3339, s.DetectedAt)

		if err := batch.Append(
			s.AnomalyID, s.AnomalyType, s.Severity, s.Metric,
			s.Observed, s.Expected, s.Deviation,
			workspaceID, windowStart, windowEnd, detectedAt,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}
