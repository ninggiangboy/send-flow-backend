package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type GetCampaignFunnelInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
}

type GetCampaignTimeSeriesInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	Interval    string
	EventType   string
}

type GetCampaignBreakdownInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	GroupBy     string
}

type GetCampaignEventsInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	EventType   string
	Provider    string
	Domain      string
	Limit       int
	Cursor      string
}

type Options struct {
	FactRepo                ports.EventFactRepository
	ClickHouseFactRepo      ports.EventFactRepository
	ProjectionRead          ports.ProjectionReadRepository
	ProjectionWrite         ports.ProjectionWriteRepository
	CampaignQueryRepo       ports.CampaignQueryRepository
	DeliverabilityQueryRepo ports.DeliverabilityQueryRepository
	ForensicQueryRepo       ports.ForensicQueryRepository
	OperationsQueryRepo     ports.OperationsQueryRepository
	TxManager               ports.TransactionManager
	OutboxWriter            ports.OutboxWriter
	AccessChecker           ports.WorkspaceAccessChecker
	IDGen                   func() (string, error)
	Clock                   func() time.Time
	Logger                  *slog.Logger
}

type Service struct {
	factRepo                ports.EventFactRepository
	clickHouseFactRepo      ports.EventFactRepository
	projectionRead          ports.ProjectionReadRepository
	projectionWrite         ports.ProjectionWriteRepository
	campaignQueryRepo       ports.CampaignQueryRepository
	deliverabilityQueryRepo ports.DeliverabilityQueryRepository
	forensicQueryRepo       ports.ForensicQueryRepository
	operationsQueryRepo     ports.OperationsQueryRepository
	txManager               ports.TransactionManager
	outboxWriter            ports.OutboxWriter
	accessChecker           ports.WorkspaceAccessChecker
	idGen                   func() (string, error)
	clock                   func() time.Time
	log                     *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		factRepo:                opts.FactRepo,
		clickHouseFactRepo:      opts.ClickHouseFactRepo,
		projectionRead:          opts.ProjectionRead,
		projectionWrite:         opts.ProjectionWrite,
		campaignQueryRepo:       opts.CampaignQueryRepo,
		deliverabilityQueryRepo: opts.DeliverabilityQueryRepo,
		forensicQueryRepo:       opts.ForensicQueryRepo,
		operationsQueryRepo:     opts.OperationsQueryRepo,
		txManager:               opts.TxManager,
		outboxWriter:            opts.OutboxWriter,
		accessChecker:           opts.AccessChecker,
		idGen:                   opts.IDGen,
		clock:                   opts.Clock,
		log:                     opts.Logger.With("service", "analytics"),
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

	var fact domain.EmailEventFact
	var created bool

	txErr := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
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

		fact = domain.EmailEventFact{
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

		created = true

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
	if txErr != nil {
		return txErr
	}

	if s.clickHouseFactRepo != nil && created {
		chErr := s.clickHouseFactRepo.Create(ctx, fact)
		if chErr != nil {
			if errors.Is(chErr, domain.ErrAnalyticsEventDuplicate) {
				s.log.Debug("duplicate analytics event in clickhouse, skipping",
					"source_event_id", input.SourceEventID,
				)
				return nil
			}
			s.log.Error("failed to write analytics fact to clickhouse",
				"source_event_id", input.SourceEventID,
				"workspace_id", input.WorkspaceID,
				"error", chErr,
			)
			return chErr
		}
	}

	return nil
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

func (s *Service) GetCampaignFunnel(ctx context.Context, input GetCampaignFunnelInput) (*domain.CampaignFunnel, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	funnel, err := s.campaignQueryRepo.GetCampaignFunnel(ctx, input.WorkspaceID, input.CampaignID, input.From, input.To)
	if err != nil {
		s.log.Error("failed to get campaign funnel",
			"workspace_id", input.WorkspaceID,
			"campaign_id", input.CampaignID,
			"error", err,
		)
		return nil, err
	}

	return funnel, nil
}

func (s *Service) GetCampaignTimeSeries(ctx context.Context, input GetCampaignTimeSeriesInput) (*domain.CampaignTimeSeriesResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if err := domain.ValidateInterval(input.Interval); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}
	if input.EventType != "" && !domain.KnownEventTypes[input.EventType] {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	ts, err := s.campaignQueryRepo.GetCampaignTimeSeries(ctx, input.WorkspaceID, input.CampaignID, input.From, input.To, input.Interval, input.EventType)
	if err != nil {
		s.log.Error("failed to get campaign time series",
			"workspace_id", input.WorkspaceID,
			"campaign_id", input.CampaignID,
			"interval", input.Interval,
			"error", err,
		)
		return nil, err
	}

	return ts, nil
}

func (s *Service) GetCampaignBreakdown(ctx context.Context, input GetCampaignBreakdownInput) (*domain.CampaignBreakdownResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if err := domain.ValidateGroupBy(input.GroupBy); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	bd, err := s.campaignQueryRepo.GetCampaignBreakdown(ctx, input.WorkspaceID, input.CampaignID, input.From, input.To, input.GroupBy)
	if err != nil {
		s.log.Error("failed to get campaign breakdown",
			"workspace_id", input.WorkspaceID,
			"campaign_id", input.CampaignID,
			"group_by", input.GroupBy,
			"error", err,
		)
		return nil, err
	}

	return bd, nil
}

type GetDeliverabilityTimeSeriesInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
	Interval    string
}

type GetDeliverabilityBreakdownInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	GroupBy     string
	Provider    string
	Domain      string
}

type GetDeliverabilityLatencyInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
}

type GetDeliverabilityIncidentsInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
}

func (s *Service) GetDeliverabilityTimeSeries(ctx context.Context, input GetDeliverabilityTimeSeriesInput) (*domain.DeliverabilityTimeSeriesResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.deliverabilityQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if input.Interval == "" {
		input.Interval = "day"
	}
	if err := domain.ValidateInterval(input.Interval); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.deliverabilityQueryRepo.GetDeliverabilityTimeSeries(ctx, input.WorkspaceID, input.From, input.To, input.Provider, input.Domain, input.Interval)
	if err != nil {
		s.log.Error("failed to get deliverability time series",
			"workspace_id", input.WorkspaceID,
			"provider", input.Provider,
			"domain", input.Domain,
			"interval", input.Interval,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (s *Service) GetDeliverabilityBreakdown(ctx context.Context, input GetDeliverabilityBreakdownInput) (*domain.DeliverabilityBreakdownResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.deliverabilityQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if input.GroupBy == "" {
		input.GroupBy = "provider"
	}
	if err := domain.ValidateGroupBy(input.GroupBy); err != nil {
		return nil, err
	}
	if input.GroupBy == "event_type" {
		return nil, fmt.Errorf("%w: group_by must be provider or recipient_domain for deliverability", domain.ErrAnalyticsQueryInvalid)
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.deliverabilityQueryRepo.GetDeliverabilityBreakdown(ctx, input.WorkspaceID, input.From, input.To, input.GroupBy, input.Provider, input.Domain)
	if err != nil {
		s.log.Error("failed to get deliverability breakdown",
			"workspace_id", input.WorkspaceID,
			"group_by", input.GroupBy,
			"provider", input.Provider,
			"domain", input.Domain,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (s *Service) GetDeliverabilityLatency(ctx context.Context, input GetDeliverabilityLatencyInput) (*domain.DeliverabilityLatencyResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.deliverabilityQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.deliverabilityQueryRepo.GetDeliverabilityLatency(ctx, input.WorkspaceID, input.From, input.To, input.Provider, input.Domain)
	if err != nil {
		s.log.Error("failed to get deliverability latency",
			"workspace_id", input.WorkspaceID,
			"provider", input.Provider,
			"domain", input.Domain,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (s *Service) GetDeliverabilityIncidents(ctx context.Context, input GetDeliverabilityIncidentsInput) (*domain.DeliverabilityIncidentResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.deliverabilityQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.deliverabilityQueryRepo.GetDeliverabilityIncidents(ctx, input.WorkspaceID, input.From, input.To, input.Provider, input.Domain)
	if err != nil {
		s.log.Error("failed to get deliverability incidents",
			"workspace_id", input.WorkspaceID,
			"provider", input.Provider,
			"domain", input.Domain,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (s *Service) GetCampaignEvents(ctx context.Context, input GetCampaignEventsInput) (*domain.CampaignEventsResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	filter := domain.CampaignQueryFilter{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		From:        input.From,
		To:          input.To,
		EventType:   input.EventType,
		Provider:    input.Provider,
		Domain:      input.Domain,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	events, err := s.campaignQueryRepo.GetCampaignEvents(ctx, filter)
	if err != nil {
		s.log.Error("failed to get campaign events",
			"workspace_id", input.WorkspaceID,
			"campaign_id", input.CampaignID,
			"error", err,
		)
		return nil, err
	}

	return events, nil
}

type SearchEventsInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	EventType   string
	Provider    string
	Domain      string
	CampaignID  string
	MessageID   string
	Limit       int
	Cursor      string
}

type GetMessageTimelineInput struct {
	WorkspaceID string
	UserID      string
	MessageID   string
}

type GetProviderEventTraceInput struct {
	WorkspaceID     string
	UserID          string
	ProviderEventID string
}

type GetCampaignIncidentTimelineInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
}

func (s *Service) SearchEvents(ctx context.Context, input SearchEventsInput) (*domain.ForensicEventsResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	filter := domain.ForensicQueryFilter{
		WorkspaceID: input.WorkspaceID,
		From:        input.From,
		To:          input.To,
		EventType:   input.EventType,
		Provider:    input.Provider,
		Domain:      input.Domain,
		CampaignID:  input.CampaignID,
		MessageID:   input.MessageID,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	if err := filter.Validate(); err != nil {
		return nil, err
	}

	result, err := s.forensicQueryRepo.SearchEvents(ctx, filter)
	if err != nil {
		s.log.Error("failed to search forensic events",
			"workspace_id", input.WorkspaceID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (s *Service) GetMessageTimeline(ctx context.Context, input GetMessageTimelineInput) (*domain.MessageTimelineResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	result, err := s.forensicQueryRepo.GetMessageTimeline(ctx, input.WorkspaceID, input.MessageID)
	if err != nil {
		s.log.Error("failed to get message timeline",
			"workspace_id", input.WorkspaceID,
			"message_id", input.MessageID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (s *Service) GetProviderEventTrace(ctx context.Context, input GetProviderEventTraceInput) (*domain.ProviderEventTrace, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	result, err := s.forensicQueryRepo.GetProviderEventTrace(ctx, input.WorkspaceID, input.ProviderEventID)
	if err != nil {
		s.log.Error("failed to get provider event trace",
			"workspace_id", input.WorkspaceID,
			"provider_event_id", input.ProviderEventID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (s *Service) GetCampaignIncidentTimeline(ctx context.Context, input GetCampaignIncidentTimelineInput) (*domain.CampaignIncidentTimelineResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if s.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}

	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.forensicQueryRepo.GetCampaignIncidentTimeline(ctx, input.WorkspaceID, input.CampaignID, input.From, input.To)
	if err != nil {
		s.log.Error("failed to get campaign incident timeline",
			"workspace_id", input.WorkspaceID,
			"campaign_id", input.CampaignID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

type GetOutboxLagInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

type GetConsumerFailuresInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

type GetDLQVolumeInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

type GetWebhookDeliveryTimeSeriesInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Status      string
	Interval    string
}

type GetWebhookReliabilityInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Target      string
}

func (s *Service) GetOutboxLag(ctx context.Context, input GetOutboxLagInput) (*domain.OutboxLagResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if s.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.operationsQueryRepo.GetOutboxLag(ctx, input.WorkspaceID, input.From, input.To, input.Source)
	if err != nil {
		s.log.Error("failed to get outbox lag",
			"workspace_id", input.WorkspaceID,
			"source", input.Source,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (s *Service) GetConsumerFailures(ctx context.Context, input GetConsumerFailuresInput) (*domain.ConsumerFailureResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if s.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.operationsQueryRepo.GetConsumerFailures(ctx, input.WorkspaceID, input.From, input.To, input.Source)
	if err != nil {
		s.log.Error("failed to get consumer failures",
			"workspace_id", input.WorkspaceID,
			"source", input.Source,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (s *Service) GetDLQVolume(ctx context.Context, input GetDLQVolumeInput) (*domain.DLQResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if s.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.operationsQueryRepo.GetDLQVolume(ctx, input.WorkspaceID, input.From, input.To, input.Source)
	if err != nil {
		s.log.Error("failed to get dlq volume",
			"workspace_id", input.WorkspaceID,
			"source", input.Source,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (s *Service) GetWebhookDeliveryTimeSeries(ctx context.Context, input GetWebhookDeliveryTimeSeriesInput) (*domain.WebhookDeliveryTimeSeriesResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if s.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}
	if input.Interval == "" {
		input.Interval = "day"
	}
	if err := domain.ValidateInterval(input.Interval); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.operationsQueryRepo.GetWebhookDeliveryTimeSeries(ctx, input.WorkspaceID, input.From, input.To, input.Status, input.Interval)
	if err != nil {
		s.log.Error("failed to get webhook delivery time series",
			"workspace_id", input.WorkspaceID,
			"status", input.Status,
			"interval", input.Interval,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (s *Service) GetWebhookReliability(ctx context.Context, input GetWebhookReliabilityInput) (*domain.WebhookReliabilityResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if s.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsQueryInvalid
	}
	if err := domain.ValidateTimeRange(input.From, input.To); err != nil {
		return nil, err
	}

	result, err := s.operationsQueryRepo.GetWebhookReliability(ctx, input.WorkspaceID, input.From, input.To, input.Target)
	if err != nil {
		s.log.Error("failed to get webhook reliability",
			"workspace_id", input.WorkspaceID,
			"target", input.Target,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}
