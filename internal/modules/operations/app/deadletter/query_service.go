package deadletter

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
	platformconstants "github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	DeadLetterRepo ports.DeadLetterRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type ListInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.DeadLetterFilter
}

type GetInput struct {
	WorkspaceID string
	UserID      string
	RecordID    string
}

type QueryService struct {
	deadLetterRepo ports.DeadLetterRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func NewQueryService(opts Options) *QueryService {
	return &QueryService{
		deadLetterRepo: opts.DeadLetterRepo,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "deadletter_query"),
	}
}

func (s *QueryService) ListRecords(ctx context.Context, input ListInput) ([]domain.DeadLetterRecord, string, error) {
	log := s.log.With("workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, platformconstants.PermissionOperationsDLQRead); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid dead letter filter", "error", err)
		return nil, "", err
	}
	records, cursor, err := s.deadLetterRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list dead letter records", "error", err)
		return nil, "", err
	}
	for i := range records {
		records[i].Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(records[i].Payload, 4096))
	}
	return records, cursor, nil
}

func (s *QueryService) GetRecord(ctx context.Context, input GetInput) (*domain.DeadLetterRecord, error) {
	log := s.log.With("workspace_id", input.WorkspaceID, "dead_letter_id", input.RecordID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, platformconstants.PermissionOperationsDLQRead); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	rec, err := s.deadLetterRepo.FindByID(ctx, input.WorkspaceID, input.RecordID)
	if err != nil {
		if errors.Is(err, domain.ErrDeadLetterRecordNotFound) {
			log.Warn("dead letter record not found")
			return nil, err
		}
		log.Error("failed to get dead letter record", "error", err)
		return nil, err
	}
	rec.Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(rec.Payload, 4096))
	return rec, nil
}
