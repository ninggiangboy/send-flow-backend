package outbox

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

type Options struct {
	OutboxRepo    ports.OutboxRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type SummaryInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.OutboxFilter
}

type ListInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.OutboxFilter
}

type GetInput struct {
	WorkspaceID string
	UserID      string
	OutboxID    string
}

type QueryService struct {
	outboxRepo    ports.OutboxRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewQueryService(opts Options) *QueryService {
	return &QueryService{
		outboxRepo:    opts.OutboxRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "outbox_query"),
	}
}

func (s *QueryService) GetSummary(ctx context.Context, input SummaryInput) (domain.OutboxSummary, error) {
	log := s.log.With("workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		if errors.Is(err, domain.ErrQueueReadDenied) || errors.Is(err, domain.ErrDLQReadDenied) || errors.Is(err, domain.ErrReplayManageDenied) {
			return domain.OutboxSummary{}, err
		}
		return domain.OutboxSummary{}, err
	}
	if input.WorkspaceID == "" {
		log.Warn("missing workspace id")
		return domain.OutboxSummary{}, domain.ErrWorkspaceRequired
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid outbox filter", "error", err)
		return domain.OutboxSummary{}, err
	}
	return s.outboxRepo.GetSummary(ctx, input.WorkspaceID, input.Filter)
}

func (s *QueryService) ListRecords(ctx context.Context, input ListInput) ([]domain.OutboxRecord, string, error) {
	log := s.log.With("workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid outbox filter", "error", err)
		return nil, "", err
	}
	records, cursor, err := s.outboxRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list outbox records", "error", err)
		return nil, "", err
	}
	for i := range records {
		records[i].Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(records[i].Payload, 4096))
		if len(records[i].Headers) > 0 && string(records[i].Headers) != "null" {
			records[i].Headers = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(records[i].Headers, 2048))
		}
	}
	return records, cursor, nil
}

func (s *QueryService) GetRecord(ctx context.Context, input GetInput) (*domain.OutboxRecord, error) {
	log := s.log.With("workspace_id", input.WorkspaceID, "outbox_id", input.OutboxID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	rec, err := s.outboxRepo.FindByID(ctx, input.WorkspaceID, input.OutboxID)
	if err != nil {
		if errors.Is(err, domain.ErrOutboxRecordNotFound) {
			log.Warn("outbox record not found")
			return nil, err
		}
		log.Error("failed to get outbox record", "error", err)
		return nil, err
	}
	rec.Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(rec.Payload, 4096))
	if len(rec.Headers) > 0 && string(rec.Headers) != "null" {
		rec.Headers = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(rec.Headers, 2048))
	}
	return rec, nil
}
