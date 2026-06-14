package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
)

type OutboxLagSampler struct {
	name   string
	svc    *analyticsapp.Service
	pgPool *pgxpool.Pool
	log    *slog.Logger
	guard  *PollingGuard
}

func NewOutboxLagSampler(svc *analyticsapp.Service, pgPool *pgxpool.Pool, log *slog.Logger, pollInterval time.Duration) *OutboxLagSampler {
	name := "analytics.outbox_lag_sampler"
	return &OutboxLagSampler{
		name:   name,
		svc:    svc,
		pgPool: pgPool,
		log:    log.With("worker", name),
		guard:  NewPollingGuard(name, pollInterval, 3, 0, log),
	}
}

func (s *OutboxLagSampler) Name() string {
	return s.name
}

func (s *OutboxLagSampler) Run(ctx context.Context) error {
	s.log.Info("starting outbox lag sampler")
	return s.guard.Run(ctx, s)
}

func (s *OutboxLagSampler) Poll(ctx context.Context) (bool, error) {
	cutoff := time.Now().Add(-1 * time.Minute)

	rows, err := s.pgPool.Query(ctx, `
		SELECT o.id, o.aggregate_type, o.event_type, COALESCE(o.workspace_id, ''), o.occurred_at
		FROM outbox_events o
		LEFT JOIN processed_event_markers m ON m.event_id = o.id
		WHERE m.event_id IS NULL
			AND o.occurred_at < $1
		ORDER BY o.occurred_at ASC
		LIMIT 1000`, cutoff)
	if err != nil {
		s.log.Error("failed to query pending outbox events", "error", err)
		return false, nil
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var eventID, aggType, evType, wsID string
		var occurredAt time.Time
		if err := rows.Scan(&eventID, &aggType, &evType, &wsID, &occurredAt); err != nil {
			s.log.Error("failed to scan outbox event row", "error", err)
			return false, nil
		}

		if err := s.svc.IngestOperationsEvent(ctx, analyticsapp.IngestOperationsEventInput{
			SourceEventID:   eventID,
			Source:          aggType,
			SourceEventType: evType,
			OperationType:   "outbox_lag",
			Status:          "pending",
			WorkspaceID:     wsID,
			OccurredAt:      occurredAt,
		}); err != nil {
			s.log.Error("failed to record outbox lag event",
				"event_id", eventID, "error", err,
			)
			return false, nil
		}
		count++
	}

	if err := rows.Err(); err != nil {
		s.log.Error("error iterating outbox event rows", "error", err)
		return false, nil
	}

	if count > 0 {
		s.log.Debug("recorded outbox lag events", "count", count)
	}
	return count > 0, nil
}
