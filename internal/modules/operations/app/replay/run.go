package replay

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
	platformconstants "github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type RunOptions struct {
	DeadLetterRepo ports.DeadLetterRepository
	OutboxRepo     ports.OutboxRepository
	ReplayJobRepo  ports.ReplayJobRepository
	OutboxWriter   ports.OutboxWriter
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Clock          func() time.Time
	Logger         *slog.Logger
}

type RunInput struct {
	WorkspaceID string
	UserID      string
	JobID       string
}

type RunHandler struct {
	deadLetterRepo ports.DeadLetterRepository
	outboxRepo     ports.OutboxRepository
	replayJobRepo  ports.ReplayJobRepository
	outboxWriter   ports.OutboxWriter
	accessChecker  ports.WorkspaceAccessChecker
	idGen          func() (string, error)
	clock          func() time.Time
	log            *slog.Logger
}

func NewRunHandler(opts RunOptions) *RunHandler {
	return &RunHandler{
		deadLetterRepo: opts.DeadLetterRepo,
		outboxRepo:     opts.OutboxRepo,
		replayJobRepo:  opts.ReplayJobRepo,
		outboxWriter:   opts.OutboxWriter,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		clock:          opts.Clock,
		log:            opts.Logger.With("usecase", "run_replay_job"),
	}
}

func (h *RunHandler) Execute(ctx context.Context, input RunInput) (*domain.ReplayJob, error) {
	log := h.log.With("workspace_id", input.WorkspaceID, "replay_job_id", input.JobID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, platformconstants.PermissionOperationsReplayManage); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}

	now := h.clock()

	if err := h.replayJobRepo.MarkRunning(ctx, input.WorkspaceID, input.JobID, now); err != nil {
		log.Error("failed to mark job running", "error", err)
		return nil, err
	}

	job, err := h.replayJobRepo.FindByID(ctx, input.WorkspaceID, input.JobID)
	if err != nil {
		log.Error("failed to find replay job", "error", err)
		return nil, err
	}

	var resultErr error
	switch job.TargetType {
	case domain.ReplayTargetDeadLetter:
		resultErr = h.executeDeadLetterReplay(ctx, job, log)
	default:
		log.Warn("unsupported target type for execution", "target_type", job.TargetType)
		resultErr = domain.ErrReplayTargetInvalid
	}

	if resultErr != nil {
		errMsg := domain.SanitizeErrorMessage(resultErr.Error())
		if errors.Is(resultErr, domain.ErrReplayTargetInvalid) || errors.Is(resultErr, domain.ErrDeadLetterRecordNotFound) {
			if err := h.replayJobRepo.MarkFailed(ctx, input.WorkspaceID, input.JobID, errMsg, now); err != nil {
				log.Error("failed to mark job failed", "error", err)
				return nil, err
			}
			job.Status = domain.ReplayJobFailed
			job.ErrorMessage = errMsg
			job.CompletedAt = &now
			log.Warn("replay job failed", "replay_job_id", input.JobID, "error", errMsg)
			return job, nil
		}
		return nil, resultErr
	}

	resultData := map[string]any{"replayed": true}
	if err := h.replayJobRepo.MarkCompleted(ctx, input.WorkspaceID, input.JobID, resultData, now); err != nil {
		log.Error("failed to mark job completed", "error", err)
		return nil, err
	}
	job.Status = domain.ReplayJobCompleted
	job.CompletedAt = &now
	job.Result, _ = json.Marshal(resultData)

	if h.outboxWriter != nil {
		if err := h.outboxWriter.Write(ctx, contracts.EventReplayJobCompletedV1, input.JobID, input.WorkspaceID, contracts.ReplayJobCompletedPayload{
			JobID:       input.JobID,
			WorkspaceID: input.WorkspaceID,
			TargetType:  string(job.TargetType),
			TargetID:    job.TargetID,
			ResultCount: 1,
		}); err != nil {
			log.Error("failed to publish replay job completed event", "error", err)
		}
	}

	log.Info("replay job completed", "replay_job_id", input.JobID)
	return job, nil
}

func (h *RunHandler) executeDeadLetterReplay(ctx context.Context, job *domain.ReplayJob, log *slog.Logger) error {
	dlq, err := h.deadLetterRepo.FindByID(ctx, job.WorkspaceID, job.TargetID)
	if err != nil {
		return err
	}

	now := h.clock()
	eventID, err := h.idGen()
	if err != nil {
		return err
	}

	replayEventType := dlq.SourceEventType
	if replayEventType == "" {
		var envelope struct {
			EventType string `json:"event_type"`
		}
		if json.Unmarshal(dlq.Payload, &envelope) == nil && envelope.EventType != "" {
			replayEventType = envelope.EventType
		}
	}
	if replayEventType == "" {
		replayEventType = "operations.replay_event.v1"
	}

	replayHeaders := json.RawMessage(
		`{"replay_of":"` + job.TargetID + `","replay_job_id":"` + job.ID + `","original_event_type":"` + replayEventType + `"}`,
	)

	replayEvent := domain.OutboxRecord{
		ID:            eventID,
		WorkspaceID:   job.WorkspaceID,
		AggregateType: contracts.AggregateReplay,
		AggregateID:   job.TargetID,
		EventType:     replayEventType,
		Payload:       dlq.Payload,
		Headers:       replayHeaders,
		OccurredAt:    now,
		CreatedAt:     now,
	}

	if err := h.outboxRepo.CreateReplayOutboxEvent(ctx, replayEvent); err != nil {
		log.Error("failed to create replay outbox event", "error", err)
		return err
	}

	log.Info("dead letter record replayed",
		"replay_job_id", job.ID,
		"dlq_id", job.TargetID,
		"original_event_type", replayEventType,
	)
	return nil
}
