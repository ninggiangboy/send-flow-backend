package worker

import (
	"context"
	"log/slog"
	"time"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

const (
	defaultSyncPollInterval = 30 * time.Second
	defaultSyncBatchSize    = 1000
	defaultSyncMaxBatches   = 5
	analyticsEmailStream    = "analytics_email_events"
)

type AnalyticsClickHouseSyncProcessor struct {
	name       string
	svc        *analyticsapp.Service
	log        *slog.Logger
	batchSize  int
	maxBatches int
	metrics    *observability.SyncMetrics
	guard      *PollingGuard
}

type AnalyticsClickHouseSyncProcessorOption func(*AnalyticsClickHouseSyncProcessor)

func WithSyncPollInterval(d time.Duration) AnalyticsClickHouseSyncProcessorOption {
	return func(p *AnalyticsClickHouseSyncProcessor) {
		p.guard = NewPollingGuard(p.name, d, 3, 0, p.log)
	}
}

func WithSyncBatchSize(n int) AnalyticsClickHouseSyncProcessorOption {
	return func(p *AnalyticsClickHouseSyncProcessor) {
		p.batchSize = n
	}
}

func WithSyncMaxBatches(n int) AnalyticsClickHouseSyncProcessorOption {
	return func(p *AnalyticsClickHouseSyncProcessor) {
		p.maxBatches = n
	}
}

func WithSyncMetrics(m *observability.SyncMetrics) AnalyticsClickHouseSyncProcessorOption {
	return func(p *AnalyticsClickHouseSyncProcessor) {
		p.metrics = m
	}
}

func NewAnalyticsClickHouseSyncProcessor(svc *analyticsapp.Service, log *slog.Logger, opts ...AnalyticsClickHouseSyncProcessorOption) *AnalyticsClickHouseSyncProcessor {
	name := "analytics.clickhouse_sync"
	p := &AnalyticsClickHouseSyncProcessor{
		name:       name,
		svc:        svc,
		log:        log.With("worker", name),
		batchSize:  defaultSyncBatchSize,
		maxBatches: defaultSyncMaxBatches,
		guard:      NewPollingGuard(name, defaultSyncPollInterval, 3, 0, log),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *AnalyticsClickHouseSyncProcessor) Name() string {
	return p.name
}

func (p *AnalyticsClickHouseSyncProcessor) Run(ctx context.Context) error {
	p.log.Info("starting clickhouse sync processor",
		"batch_size", p.batchSize,
		"max_batches", p.maxBatches,
	)
	return p.guard.Run(ctx, p)
}

func (p *AnalyticsClickHouseSyncProcessor) Poll(ctx context.Context) (bool, error) {
	start := time.Now()
	batchesProcessed := 0
	totalSynced := 0

	for batchesProcessed < p.maxBatches {
		select {
		case <-ctx.Done():
			p.log.Info("sync cancelled, context done")
			return totalSynced > 0, nil
		default:
		}

		synced, err := p.svc.SyncFactsToClickHouse(ctx, analyticsapp.SyncFactsToClickHouseInput{
			StreamName: analyticsEmailStream,
			BatchSize:  p.batchSize,
		})
		if err != nil {
			p.log.Error("failed to sync facts to clickhouse",
				"stream_name", analyticsEmailStream,
				"batch_size", p.batchSize,
				"batches_processed", batchesProcessed,
				"error", err,
			)
			if p.metrics != nil {
				p.metrics.RecordBatch(0, time.Since(start).Seconds(), err)
			}
			return totalSynced > 0, nil
		}

		if synced.SyncedCount == 0 {
			break
		}

		totalSynced += synced.SyncedCount
		batchesProcessed++

		if p.metrics != nil {
			p.metrics.RecordBatch(synced.SyncedCount, time.Since(start).Seconds(), nil)
			p.metrics.SetLag(float64(synced.LagSeconds))
		}

		p.log.Info("synced batch to clickhouse",
			"stream_name", analyticsEmailStream,
			"batch_size", synced.SyncedCount,
			"last_fact_id", synced.LastFactID,
		)
	}

	duration := time.Since(start)
	if totalSynced > 0 || batchesProcessed > 0 {
		p.log.Info("clickhouse sync cycle complete",
			"stream_name", analyticsEmailStream,
			"batches", batchesProcessed,
			"synced_count", totalSynced,
			"duration_ms", duration.Milliseconds(),
		)
	}
	return totalSynced > 0, nil
}
