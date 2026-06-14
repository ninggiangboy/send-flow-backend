package ingestion

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

type Options struct {
	FactRepo              ports.EventFactRepository
	ProjectionWrite       ports.ProjectionWriteRepository
	TxManager             ports.TransactionManager
	OutboxWriter          ports.OutboxWriter
	IDGen                 func() (string, error)
	Clock                 func() time.Time
	AccessChecker         ports.WorkspaceAccessChecker
	OperationsEventWriter ports.OperationsEventWriter
	Logger                *slog.Logger
}

type Handler struct {
	factRepo              ports.EventFactRepository
	projectionWrite       ports.ProjectionWriteRepository
	txManager             ports.TransactionManager
	outboxWriter          ports.OutboxWriter
	idGen                 func() (string, error)
	clock                 func() time.Time
	accessChecker         ports.WorkspaceAccessChecker
	operationsEventWriter ports.OperationsEventWriter
	log                   *slog.Logger
}

type IngestFactCommand struct {
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

type IngestOpsCommand struct {
	Source          string
	SourceEventID   string
	SourceEventType string
	OperationType   string
	Status          string
	WorkspaceID     string
	ErrorType       string
	Consumer        string
	Target          string
	Metadata        map[string]any
	OccurredAt      time.Time
}

func New(opts Options) *Handler {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		factRepo:              opts.FactRepo,
		projectionWrite:       opts.ProjectionWrite,
		txManager:             opts.TxManager,
		outboxWriter:          opts.OutboxWriter,
		idGen:                 opts.IDGen,
		clock:                 opts.Clock,
		accessChecker:         opts.AccessChecker,
		operationsEventWriter: opts.OperationsEventWriter,
		log:                   opts.Logger.With("service", "analytics", "handler", "ingestion"),
	}
}

func (h *Handler) ExecuteIngestEmailEventFact(ctx context.Context, cmd IngestFactCommand) error {
	cmd.WorkspaceID = domain.NormalizeString(cmd.WorkspaceID)
	cmd.CampaignID = domain.NormalizeString(cmd.CampaignID)
	cmd.Provider = domain.NormalizeString(cmd.Provider)
	cmd.RecipientDomain = domain.NormalizeString(cmd.RecipientDomain)

	if err := domain.ValidateFact(cmd.WorkspaceID, cmd.SourceEventID, cmd.CanonicalType, cmd.OccurredAt, cmd.ReceivedAt); err != nil {
		return err
	}

	var fact domain.EmailEventFact

	txErr := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		existing, err := h.factRepo.FindBySourceEventID(txCtx, cmd.SourceEventID)
		if err != nil && !errors.Is(err, domain.ErrAnalyticsProjectionNotFound) {
			h.log.Error("failed to check existing fact",
				"source_event_id", cmd.SourceEventID,
				"workspace_id", cmd.WorkspaceID,
				"error", err,
			)
			return err
		}
		if existing != nil {
			h.log.Debug("duplicate analytics event, skipping",
				"source_event_id", cmd.SourceEventID,
				"workspace_id", cmd.WorkspaceID,
				"event_type", cmd.CanonicalType,
			)
			return nil
		}

		factID, err := h.idGen()
		if err != nil {
			return err
		}
		now := h.clock()

		fact = domain.EmailEventFact{
			ID:                factID,
			SourceEventID:     cmd.SourceEventID,
			SourceEventType:   cmd.SourceEventType,
			WorkspaceID:       cmd.WorkspaceID,
			CampaignID:        cmd.CampaignID,
			MessageID:         cmd.MessageID,
			Provider:          cmd.Provider,
			ProviderMessageID: cmd.ProviderMessageID,
			ProviderEventID:   cmd.ProviderEventID,
			EventType:         cmd.CanonicalType,
			RecipientDomain:   cmd.RecipientDomain,
			OccurredAt:        cmd.OccurredAt,
			ReceivedAt:        cmd.ReceivedAt,
			Metadata:          cmd.Metadata,
			CreatedAt:         now,
		}

		if err := h.factRepo.Create(txCtx, fact); err != nil {
			if err == domain.ErrAnalyticsEventDuplicate {
				h.log.Debug("duplicate analytics event (race), skipping",
					"source_event_id", cmd.SourceEventID,
				)
				return nil
			}
			h.log.Error("failed to create analytics fact",
				"source_event_id", cmd.SourceEventID,
				"workspace_id", cmd.WorkspaceID,
				"error", err,
			)
			return err
		}

		if err := h.projectionWrite.IncrementWorkspaceOverview(txCtx, cmd.WorkspaceID, cmd.CanonicalType, cmd.OccurredAt); err != nil {
			h.log.Error("failed to increment workspace overview",
				"workspace_id", cmd.WorkspaceID,
				"event_type", cmd.CanonicalType,
				"error", err,
			)
			return err
		}

		if cmd.CampaignID != "" {
			if err := h.projectionWrite.IncrementCampaignSummary(txCtx, cmd.WorkspaceID, cmd.CampaignID, cmd.CanonicalType, cmd.OccurredAt); err != nil {
				h.log.Error("failed to increment campaign summary",
					"workspace_id", cmd.WorkspaceID,
					"campaign_id", cmd.CampaignID,
					"event_type", cmd.CanonicalType,
					"error", err,
				)
				return err
			}
		}

		if cmd.Provider != "" || cmd.RecipientDomain != "" {
			prov := cmd.Provider
			dom := cmd.RecipientDomain
			if err := h.projectionWrite.IncrementDeliverability(txCtx, cmd.WorkspaceID, prov, dom, cmd.CanonicalType, cmd.OccurredAt); err != nil {
				h.log.Error("failed to increment deliverability projection",
					"workspace_id", cmd.WorkspaceID,
					"provider", prov,
					"recipient_domain", dom,
					"event_type", cmd.CanonicalType,
					"error", err,
				)
				return err
			}
		}

		if h.outboxWriter != nil {
			payload, err := json.Marshal(analyticscontracts.ProjectionUpdatedPayload{
				WorkspaceID:    cmd.WorkspaceID,
				ProjectionType: "workspace_overview",
				ProjectionID:   cmd.WorkspaceID,
				EventType:      cmd.CanonicalType,
				LastEventAt:    cmd.OccurredAt.Format(time.RFC3339),
				LastUpdatedAt:  now.Format(time.RFC3339),
			})
			if err != nil {
				return err
			}
			eventID, err := h.idGen()
			if err != nil {
				return err
			}
			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "analytics",
				AggregateID:   cmd.WorkspaceID,
				EventType:     analyticscontracts.EventProjectionUpdatedV1,
				Payload:       payload,
				WorkspaceID:   cmd.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				h.log.Error("failed to write outbox event",
					"workspace_id", cmd.WorkspaceID,
					"error", err,
				)
				return err
			}
		}

		h.log.Info("analytics event fact recorded",
			"fact_id", factID,
			"source_event_id", cmd.SourceEventID,
			"workspace_id", cmd.WorkspaceID,
			"campaign_id", cmd.CampaignID,
			"message_id", cmd.MessageID,
			"event_type", cmd.CanonicalType,
		)

		return nil
	})
	if txErr != nil {
		return txErr
	}

	return nil
}

func (h *Handler) ExecuteIngestOperationsEvent(ctx context.Context, cmd IngestOpsCommand) error {
	if h.operationsEventWriter == nil {
		return nil
	}
	sourceEventID := cmd.SourceEventID
	if sourceEventID == "" {
		sourceEventID = cmd.Source + "_" + cmd.OperationType + "_" + cmd.OccurredAt.Format(time.RFC3339Nano)
	}
	return h.operationsEventWriter.Create(ctx, ports.OperationsEvent{
		SourceEventID:   sourceEventID,
		Source:          cmd.Source,
		SourceEventType: cmd.SourceEventType,
		OperationType:   cmd.OperationType,
		Status:          cmd.Status,
		WorkspaceID:     cmd.WorkspaceID,
		ErrorType:       cmd.ErrorType,
		Consumer:        cmd.Consumer,
		Target:          cmd.Target,
		Metadata:        cmd.Metadata,
		OccurredAt:      cmd.OccurredAt,
	})
}
