package worker

import (
	"context"
	"log/slog"
	"time"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
)

type AnalyticsAnomalyProcessor struct {
	name         string
	analyticsSvc *analyticsapp.Service
	log          *slog.Logger
	pollInterval time.Duration
}

func NewAnalyticsAnomalyProcessor(svc *analyticsapp.Service, log *slog.Logger, pollInterval time.Duration) *AnalyticsAnomalyProcessor {
	return &AnalyticsAnomalyProcessor{
		name:         "analytics.detect_anomalies",
		analyticsSvc: svc,
		log:          log.With("worker", "analytics.detect_anomalies"),
		pollInterval: pollInterval,
	}
}

func (p *AnalyticsAnomalyProcessor) Name() string {
	return p.name
}

func (p *AnalyticsAnomalyProcessor) Run(ctx context.Context) error {
	p.log.Info("starting anomaly detection processor",
		"poll_interval", p.pollInterval,
	)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("anomaly detection processor stopped")
			return nil
		case <-ticker.C:
			p.processOnce(ctx)
		}
	}
}

func (p *AnalyticsAnomalyProcessor) processOnce(ctx context.Context) {
	p.log.Info("running anomaly detection cycle")

	if err := p.analyticsSvc.DetectAnomaliesAllWorkspaces(ctx); err != nil {
		p.log.Error("anomaly detection cycle failed", "error", err)
	}
}
