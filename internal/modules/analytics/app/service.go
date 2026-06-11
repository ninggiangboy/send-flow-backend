package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	analyticscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type IngestEmailEventFactInput struct {
	SourceEventID     string
	SourceEventType   string
	WorkspaceID       string
	CampaignID        string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	CanonicalType     string
	RecipientDomain   string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	Metadata          map[string]any
}

type GetDashboardOverviewInput struct {
	WorkspaceID string
	UserID      string
}

type GetCampaignAnalyticsInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
}

type GetDeliverabilityInput struct {
	WorkspaceID     string
	Provider        string
	RecipientDomain string
	UserID          string
}

type Options struct {
	FactRepo        ports.EventFactRepository
	ProjectionRead  ports.ProjectionReadRepository
	ProjectionWrite ports.ProjectionWriteRepository
	TxManager       ports.TransactionManager
	OutboxWriter    ports.OutboxWriter
	AccessChecker   ports.WorkspaceAccessChecker
	IDGen           func() (string, error)
	Clock           func() time.Time
	Logger          *slog.Logger
}

type Service struct {
	factRepo        ports.EventFactRepository
	projectionRead  ports.ProjectionReadRepository
	projectionWrite ports.ProjectionWriteRepository
	txManager       ports.TransactionManager
	outboxWriter    ports.OutboxWriter
	accessChecker   ports.WorkspaceAccessChecker
	idGen           func() (string, error)
	clock           func() time.Time
	log             *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		factRepo:        opts.FactRepo,
		projectionRead:  opts.ProjectionRead,
		projectionWrite: opts.ProjectionWrite,
		txManager:       opts.TxManager,
		outboxWriter:    opts.OutboxWriter,
		accessChecker:   opts.AccessChecker,
		idGen:           opts.IDGen,
		clock:           opts.Clock,
		log:             opts.Logger.With("service", "analytics"),
	}
}

func (s *Service) IngestEmailEventFact(ctx context.Context, input IngestEmailEventFactInput) error {
	input.WorkspaceID = domain.NormalizeString(input.WorkspaceID)
	input.CampaignID = domain.NormalizeString(input.CampaignID)
	input.Provider = domain.NormalizeString(input.Provider)
	input.RecipientDomain = domain.NormalizeString(input.RecipientDomain)

	if err := domain.ValidateFact(input.WorkspaceID, input.SourceEventID, input.CanonicalType, input.OccurredAt, input.ReceivedAt); err != nil {
		return err
	}

	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		existing, err := s.factRepo.FindBySourceEventID(txCtx, input.SourceEventID)
		if err != nil && !errors.Is(err, domain.ErrAnalyticsProjectionNotFound) {
			s.log.Error("failed to check existing fact",
				"source_event_id", input.SourceEventID,
				"workspace_id", input.WorkspaceID,
				"error", err,
			)
			return err
		}
		if existing != nil {
			s.log.Debug("duplicate analytics event, skipping",
				"source_event_id", input.SourceEventID,
				"workspace_id", input.WorkspaceID,
				"event_type", input.CanonicalType,
			)
			return nil
		}

		factID, err := s.idGen()
		if err != nil {
			return err
		}
		now := s.clock()

		fact := domain.EmailEventFact{
			ID:                factID,
			SourceEventID:     input.SourceEventID,
			SourceEventType:   input.SourceEventType,
			WorkspaceID:       input.WorkspaceID,
			CampaignID:        input.CampaignID,
			MessageID:         input.MessageID,
			Provider:          input.Provider,
			ProviderMessageID: input.ProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			EventType:         input.CanonicalType,
			RecipientDomain:   input.RecipientDomain,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
			Metadata:          input.Metadata,
			CreatedAt:         now,
		}

		if err := s.factRepo.Create(txCtx, fact); err != nil {
			if err == domain.ErrAnalyticsEventDuplicate {
				s.log.Debug("duplicate analytics event (race), skipping",
					"source_event_id", input.SourceEventID,
				)
				return nil
			}
			s.log.Error("failed to create analytics fact",
				"source_event_id", input.SourceEventID,
				"workspace_id", input.WorkspaceID,
				"error", err,
			)
			return err
		}

		if err := s.projectionWrite.IncrementWorkspaceOverview(txCtx, input.WorkspaceID, input.CanonicalType, input.OccurredAt); err != nil {
			s.log.Error("failed to increment workspace overview",
				"workspace_id", input.WorkspaceID,
				"event_type", input.CanonicalType,
				"error", err,
			)
			return err
		}

		if input.CampaignID != "" {
			if err := s.projectionWrite.IncrementCampaignSummary(txCtx, input.WorkspaceID, input.CampaignID, input.CanonicalType, input.OccurredAt); err != nil {
				s.log.Error("failed to increment campaign summary",
					"workspace_id", input.WorkspaceID,
					"campaign_id", input.CampaignID,
					"event_type", input.CanonicalType,
					"error", err,
				)
				return err
			}
		}

		if input.Provider != "" || input.RecipientDomain != "" {
			prov := input.Provider
			dom := input.RecipientDomain
			if err := s.projectionWrite.IncrementDeliverability(txCtx, input.WorkspaceID, prov, dom, input.CanonicalType, input.OccurredAt); err != nil {
				s.log.Error("failed to increment deliverability projection",
					"workspace_id", input.WorkspaceID,
					"provider", prov,
					"recipient_domain", dom,
					"event_type", input.CanonicalType,
					"error", err,
				)
				return err
			}
		}

		if s.outboxWriter != nil {
			payload, err := json.Marshal(analyticscontracts.ProjectionUpdatedPayload{
				WorkspaceID:    input.WorkspaceID,
				ProjectionType: "workspace_overview",
				ProjectionID:   input.WorkspaceID,
				EventType:      input.CanonicalType,
				LastEventAt:    input.OccurredAt.Format(time.RFC3339),
				LastUpdatedAt:  now.Format(time.RFC3339),
			})
			if err != nil {
				return err
			}
			eventID, err := s.idGen()
			if err != nil {
				return err
			}
			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "analytics",
				AggregateID:   input.WorkspaceID,
				EventType:     analyticscontracts.EventProjectionUpdatedV1,
				Payload:       payload,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				s.log.Error("failed to write outbox event",
					"workspace_id", input.WorkspaceID,
					"error", err,
				)
				return err
			}
		}

		s.log.Info("analytics event fact recorded",
			"fact_id", factID,
			"source_event_id", input.SourceEventID,
			"workspace_id", input.WorkspaceID,
			"campaign_id", input.CampaignID,
			"message_id", input.MessageID,
			"event_type", input.CanonicalType,
		)

		return nil
	})
}

func (s *Service) GetDashboardOverview(ctx context.Context, input GetDashboardOverviewInput) (*domain.DashboardOverview, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	overview, err := s.projectionRead.GetWorkspaceOverview(ctx, input.WorkspaceID)
	if err != nil {
		if err == domain.ErrAnalyticsProjectionNotFound {
			return &domain.DashboardOverview{
				Status:      "pending",
				WorkspaceID: input.WorkspaceID,
			}, nil
		}
		s.log.Error("failed to get workspace overview",
			"workspace_id", input.WorkspaceID,
			"error", err,
		)
		return nil, err
	}

	return &domain.DashboardOverview{
		Status:              "ready",
		WorkspaceID:         overview.WorkspaceID,
		QueuedCount:         overview.QueuedCount,
		AcceptedCount:       overview.AcceptedCount,
		DeliveredCount:      overview.DeliveredCount,
		BouncedCount:        overview.BouncedCount,
		ComplainedCount:     overview.ComplainedCount,
		OpenedCount:         overview.OpenedCount,
		ClickedCount:        overview.ClickedCount,
		UnsubscribedCount:   overview.UnsubscribedCount,
		RetryScheduledCount: overview.RetryScheduledCount,
		LastEventAt:         overview.LastEventAt,
		LastUpdatedAt:       overview.LastUpdatedAt,
	}, nil
}

func (s *Service) GetCampaignAnalytics(ctx context.Context, input GetCampaignAnalyticsInput) (*domain.CampaignAnalytics, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	summary, err := s.projectionRead.GetCampaignSummary(ctx, input.WorkspaceID, input.CampaignID)
	if err != nil {
		if err == domain.ErrAnalyticsProjectionNotFound {
			return &domain.CampaignAnalytics{
				Status:      "pending",
				WorkspaceID: input.WorkspaceID,
				CampaignID:  input.CampaignID,
			}, nil
		}
		s.log.Error("failed to get campaign summary",
			"workspace_id", input.WorkspaceID,
			"campaign_id", input.CampaignID,
			"error", err,
		)
		return nil, err
	}

	delivered := summary.DeliveredCount
	accepted := summary.AcceptedCount

	return &domain.CampaignAnalytics{
		Status:              "ready",
		WorkspaceID:         summary.WorkspaceID,
		CampaignID:          summary.CampaignID,
		QueuedCount:         summary.QueuedCount,
		AcceptedCount:       summary.AcceptedCount,
		DeliveredCount:      summary.DeliveredCount,
		BouncedCount:        summary.BouncedCount,
		ComplainedCount:     summary.ComplainedCount,
		OpenedCount:         summary.OpenedCount,
		ClickedCount:        summary.ClickedCount,
		UnsubscribedCount:   summary.UnsubscribedCount,
		RetryScheduledCount: summary.RetryScheduledCount,
		DeliveryRate:        domain.ComputeRate(delivered, accepted),
		BounceRate:          domain.ComputeRate(summary.BouncedCount, delivered),
		ComplaintRate:       domain.ComputeRate(summary.ComplainedCount, delivered),
		OpenRate:            domain.ComputeRate(summary.OpenedCount, delivered),
		ClickRate:           domain.ComputeRate(summary.ClickedCount, delivered),
		UnsubscribeRate:     domain.ComputeRate(summary.UnsubscribedCount, delivered),
		LastEventAt:         summary.LastEventAt,
		LastUpdatedAt:       summary.LastUpdatedAt,
	}, nil
}

func (s *Service) GetDeliverability(ctx context.Context, input GetDeliverabilityInput) (*domain.DeliverabilityResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	filter := domain.DeliverabilityFilter{
		Provider:        domain.NormalizeString(input.Provider),
		RecipientDomain: domain.NormalizeString(input.RecipientDomain),
	}

	projections, err := s.projectionRead.ListDeliverability(ctx, input.WorkspaceID, filter)
	if err != nil {
		s.log.Error("failed to list deliverability projections",
			"workspace_id", input.WorkspaceID,
			"error", err,
		)
		return nil, err
	}

	if len(projections) == 0 {
		return &domain.DeliverabilityResult{
			Status: "pending",
			Items:  nil,
		}, nil
	}

	rows := make([]domain.DeliverabilityRow, 0, len(projections))
	for _, p := range projections {
		rows = append(rows, domain.DeliverabilityRow{
			Provider:        p.Provider,
			RecipientDomain: p.RecipientDomain,
			DeliveredCount:  p.DeliveredCount,
			BouncedCount:    p.BouncedCount,
			ComplainedCount: p.ComplainedCount,
			OpenedCount:     p.OpenedCount,
			ClickedCount:    p.ClickedCount,
			BounceRate:      domain.ComputeRate(p.BouncedCount, p.DeliveredCount),
			ComplaintRate:   domain.ComputeRate(p.ComplainedCount, p.DeliveredCount),
			LastEventAt:     p.LastEventAt,
			LastUpdatedAt:   p.LastUpdatedAt,
		})
	}

	return &domain.DeliverabilityResult{
		Status: "ready",
		Items:  rows,
	}, nil
}
