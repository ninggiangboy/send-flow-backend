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
	guard        *PollingGuard
}

func NewAnalyticsAnomalyProcessor(svc *analyticsapp.Service, log *slog.Logger, pollInterval time.Duration) *AnalyticsAnomalyProcessor {
	name := "analytics.detect_anomalies"
	return &AnalyticsAnomalyProcessor{
		name:         name,
		analyticsSvc: svc,
		log:          log.With("worker", name),
		guard:        NewPollingGuard(name, pollInterval, 3, 0, log),
	}
}

func (p *AnalyticsAnomalyProcessor) Name() string {
	return p.name
}

func (p *AnalyticsAnomalyProcessor) Run(ctx context.Context) error {
	p.log.Info("starting anomaly detection processor")
	return p.guard.Run(ctx, p)
}

func (p *AnalyticsAnomalyProcessor) Poll(ctx context.Context) (bool, error) {
	p.log.Info("running anomaly detection cycle")

	if err := p.analyticsSvc.DetectAnomaliesAllWorkspaces(ctx); err != nil {
		p.log.Error("anomaly detection cycle failed", "error", err)
		return false, nil
	}
	return true, nil
}
