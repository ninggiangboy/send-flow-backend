package createreplayjob

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

type Options struct {
	DeadLetterRepo ports.DeadLetterRepository
	OutboxRepo     ports.OutboxRepository
	ReplayJobRepo  ports.ReplayJobRepository
	OutboxWriter   ports.OutboxWriter
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Clock          func() time.Time
	Logger         *slog.Logger
}

type Handler struct {
	deadLetterRepo ports.DeadLetterRepository
	outboxRepo     ports.OutboxRepository
	replayJobRepo  ports.ReplayJobRepository
	outboxWriter   ports.OutboxWriter
	accessChecker  ports.WorkspaceAccessChecker
	idGen          func() (string, error)
	clock          func() time.Time
	log            *slog.Logger
}

type Input struct {
	WorkspaceID string
	UserID      string
	TargetType  domain.ReplayTargetType
	TargetID    string
	Source      string
	Reason      string
	Filter      json.RawMessage
}

func New(opts Options) *Handler {
	return &Handler{
		deadLetterRepo: opts.DeadLetterRepo,
		outboxRepo:     opts.OutboxRepo,
		replayJobRepo:  opts.ReplayJobRepo,
		outboxWriter:   opts.OutboxWriter,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		clock:          opts.Clock,
		log:            opts.Logger.With("usecase", "create_replay_job"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*domain.ReplayJob, error) {
	log := h.log.With("workspace_id", input.WorkspaceID, "target_type", input.TargetType, "target_id", input.TargetID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	if !domain.IsValidTargetType(input.TargetType) {
		log.Warn("invalid target type", "target_type", input.TargetType)
		return nil, domain.ErrReplayTargetInvalid
	}
	if input.TargetID == "" {
		log.Warn("missing target id")
		return nil, domain.ErrReplayTargetInvalid
	}
	if len(input.Reason) > 500 {
		input.Reason = input.Reason[:500]
	}

	now := h.clock()
	jobID, err := h.idGen()
	if err != nil {
		log.Error("failed to generate job id", "error", err)
		return nil, err
	}

	if input.Filter == nil {
		input.Filter = json.RawMessage(`{}`)
	}

	job := domain.ReplayJob{
		ID:                jobID,
		WorkspaceID:       input.WorkspaceID,
		TargetType:        input.TargetType,
		TargetID:          input.TargetID,
		Source:            input.Source,
		Status:            domain.ReplayJobQueued,
		RequestedByUserID: input.UserID,
		Reason:            input.Reason,
		Filter:            input.Filter,
		Result:            json.RawMessage(`{}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	var targetWorkspaceID string
	switch input.TargetType {
	case domain.ReplayTargetDeadLetter:
		dlq, err := h.deadLetterRepo.FindByID(ctx, input.WorkspaceID, input.TargetID)
		if err != nil {
			if errors.Is(err, domain.ErrDeadLetterRecordNotFound) {
				log.Warn("dead letter record not found for replay")
				return nil, domain.ErrReplayTargetInvalid
			}
			log.Error("failed to get dead letter record", "error", err)
			return nil, err
		}
		targetWorkspaceID = dlq.WorkspaceID
	case domain.ReplayTargetOutbox:
		outbox, err := h.outboxRepo.FindByID(ctx, input.WorkspaceID, input.TargetID)
		if err != nil {
			if errors.Is(err, domain.ErrOutboxRecordNotFound) {
				log.Warn("outbox record not found for replay")
				return nil, domain.ErrReplayTargetInvalid
			}
			log.Error("failed to get outbox record", "error", err)
			return nil, err
		}
		targetWorkspaceID = outbox.WorkspaceID
	}

	if targetWorkspaceID != input.WorkspaceID {
		log.Warn("target workspace mismatch")
		return nil, domain.ErrReplayTargetInvalid
	}

	job.Source = input.Source

	if err := h.replayJobRepo.Create(ctx, job); err != nil {
		log.Error("failed to create replay job", "error", err)
		return nil, err
	}

	if h.outboxWriter != nil {
		if err := h.outboxWriter.Write(ctx, contracts.EventReplayJobCreatedV1, jobID, input.WorkspaceID, contracts.ReplayJobCreatedPayload{
			JobID:       jobID,
			WorkspaceID: input.WorkspaceID,
			TargetType:  string(input.TargetType),
			TargetID:    input.TargetID,
			Reason:      input.Reason,
			RequestedBy: input.UserID,
		}); err != nil {
			log.Error("failed to publish replay job created event", "error", err)
		}
	}

	log.Info("replay job created",
		"replay_job_id", jobID,
		"target_type", input.TargetType,
		"target_id", input.TargetID,
		"source", input.Source,
	)
	return &job, nil
}
