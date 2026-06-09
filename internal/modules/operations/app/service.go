package app

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
	OutboxRepo     ports.OutboxRepository
	DeadLetterRepo ports.DeadLetterRepository
	ReplayJobRepo  ports.ReplayJobRepository
	TxManager      ports.TransactionManager
	OutboxWriter   ports.OutboxWriter
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Clock          func() time.Time
	Logger         *slog.Logger
}

type Service struct {
	outboxRepo     ports.OutboxRepository
	deadLetterRepo ports.DeadLetterRepository
	replayJobRepo  ports.ReplayJobRepository
	txManager      ports.TransactionManager
	outboxWriter   ports.OutboxWriter
	accessChecker  ports.WorkspaceAccessChecker
	idGen          func() (string, error)
	clock          func() time.Time
	log            *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		outboxRepo:     opts.OutboxRepo,
		deadLetterRepo: opts.DeadLetterRepo,
		replayJobRepo:  opts.ReplayJobRepo,
		txManager:      opts.TxManager,
		outboxWriter:   opts.OutboxWriter,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		clock:          opts.Clock,
		log:            opts.Logger.With("module", "operations"),
	}
}

type GetOutboxSummaryInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.OutboxFilter
}

func (s *Service) GetOutboxSummary(ctx context.Context, input GetOutboxSummaryInput) (domain.OutboxSummary, error) {
	log := s.log.With("usecase", "get_outbox_summary", "workspace_id", input.WorkspaceID)
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
	if err := validateOutboxFilter(input.Filter); err != nil {
		log.Warn("invalid outbox filter", "error", err)
		return domain.OutboxSummary{}, err
	}
	summary, err := s.outboxRepo.GetSummary(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to get outbox summary", "error", err)
		return domain.OutboxSummary{}, err
	}
	return summary, nil
}

type ListOutboxRecordsInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.OutboxFilter
}

func (s *Service) ListOutboxRecords(ctx context.Context, input ListOutboxRecordsInput) ([]domain.OutboxRecord, string, error) {
	log := s.log.With("usecase", "list_outbox_records", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if err := validateOutboxFilter(input.Filter); err != nil {
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

type GetOutboxRecordInput struct {
	WorkspaceID string
	UserID      string
	OutboxID    string
}

func (s *Service) GetOutboxRecord(ctx context.Context, input GetOutboxRecordInput) (*domain.OutboxRecord, error) {
	log := s.log.With("usecase", "get_outbox_record", "workspace_id", input.WorkspaceID, "outbox_id", input.OutboxID)
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

type ListDeadLetterRecordsInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.DeadLetterFilter
}

func (s *Service) ListDeadLetterRecords(ctx context.Context, input ListDeadLetterRecordsInput) ([]domain.DeadLetterRecord, string, error) {
	log := s.log.With("usecase", "list_dead_letter_records", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.dlq.read"); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if err := validateDeadLetterFilter(input.Filter); err != nil {
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

type GetDeadLetterRecordInput struct {
	WorkspaceID string
	UserID      string
	RecordID    string
}

func (s *Service) GetDeadLetterRecord(ctx context.Context, input GetDeadLetterRecordInput) (*domain.DeadLetterRecord, error) {
	log := s.log.With("usecase", "get_dead_letter_record", "workspace_id", input.WorkspaceID, "dead_letter_id", input.RecordID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.dlq.read"); err != nil {
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

type CreateReplayJobInput struct {
	WorkspaceID string
	UserID      string
	TargetType  domain.ReplayTargetType
	TargetID    string
	Source      string
	Reason      string
	Filter      json.RawMessage
}

func (s *Service) CreateReplayJob(ctx context.Context, input CreateReplayJobInput) (*domain.ReplayJob, error) {
	log := s.log.With("usecase", "create_replay_job", "workspace_id", input.WorkspaceID, "target_type", input.TargetType, "target_id", input.TargetID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
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

	now := s.clock()
	jobID, err := s.idGen()
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
		dlq, err := s.deadLetterRepo.FindByID(ctx, input.WorkspaceID, input.TargetID)
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
		outbox, err := s.outboxRepo.FindByID(ctx, input.WorkspaceID, input.TargetID)
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

	if err := s.replayJobRepo.Create(ctx, job); err != nil {
		log.Error("failed to create replay job", "error", err)
		return nil, err
	}

	if s.outboxWriter != nil {
		if err := s.outboxWriter.Write(ctx, contracts.EventReplayJobCreatedV1, jobID, input.WorkspaceID, contracts.ReplayJobCreatedPayload{
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

type RunReplayJobInput struct {
	WorkspaceID string
	UserID      string
	JobID       string
}

func (s *Service) RunReplayJob(ctx context.Context, input RunReplayJobInput) (*domain.ReplayJob, error) {
	log := s.log.With("usecase", "run_replay_job", "workspace_id", input.WorkspaceID, "replay_job_id", input.JobID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}

	now := s.clock()

	if err := s.replayJobRepo.MarkRunning(ctx, input.WorkspaceID, input.JobID, now); err != nil {
		log.Error("failed to mark job running", "error", err)
		return nil, err
	}

	job, err := s.replayJobRepo.FindByID(ctx, input.WorkspaceID, input.JobID)
	if err != nil {
		log.Error("failed to find replay job", "error", err)
		return nil, err
	}

	var resultErr error
	switch job.TargetType {
	case domain.ReplayTargetDeadLetter:
		resultErr = s.executeDeadLetterReplay(ctx, job, log)
	default:
		log.Warn("unsupported target type for execution", "target_type", job.TargetType)
		resultErr = domain.ErrReplayTargetInvalid
	}

	if resultErr != nil {
		errMsg := domain.SanitizeErrorMessage(resultErr.Error())
		if errors.Is(resultErr, domain.ErrReplayTargetInvalid) || errors.Is(resultErr, domain.ErrDeadLetterRecordNotFound) {
			if err := s.replayJobRepo.MarkFailed(ctx, input.WorkspaceID, input.JobID, errMsg, now); err != nil {
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

	result := map[string]any{"replayed": true}
	if err := s.replayJobRepo.MarkCompleted(ctx, input.WorkspaceID, input.JobID, result, now); err != nil {
		log.Error("failed to mark job completed", "error", err)
		return nil, err
	}
	job.Status = domain.ReplayJobCompleted
	job.CompletedAt = &now
	job.Result, _ = json.Marshal(result)

	if s.outboxWriter != nil {
		if err := s.outboxWriter.Write(ctx, contracts.EventReplayJobCompletedV1, input.JobID, input.WorkspaceID, contracts.ReplayJobCompletedPayload{
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

func (s *Service) executeDeadLetterReplay(ctx context.Context, job *domain.ReplayJob, log *slog.Logger) error {
	dlq, err := s.deadLetterRepo.FindByID(ctx, job.WorkspaceID, job.TargetID)
	if err != nil {
		return err
	}

	now := s.clock()
	eventID, err := s.idGen()
	if err != nil {
		return err
	}

	replayHeaders := json.RawMessage(`{"replay_of":"` + job.TargetID + `","replay_job_id":"` + job.ID + `"}`)

	replayEvent := domain.OutboxRecord{
		ID:            eventID,
		WorkspaceID:   job.WorkspaceID,
		AggregateType: "replay",
		AggregateID:   job.TargetID,
		EventType:     "operations.replay_event.v1",
		Payload:       dlq.Payload,
		Headers:       replayHeaders,
		OccurredAt:    now,
		CreatedAt:     now,
	}

	if err := s.outboxRepo.CreateReplayOutboxEvent(ctx, replayEvent); err != nil {
		log.Error("failed to create replay outbox event", "error", err)
		return err
	}

	return nil
}

type GetReplayJobInput struct {
	WorkspaceID string
	UserID      string
	JobID       string
}

func (s *Service) GetReplayJob(ctx context.Context, input GetReplayJobInput) (*domain.ReplayJob, error) {
	log := s.log.With("usecase", "get_replay_job", "workspace_id", input.WorkspaceID, "replay_job_id", input.JobID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	job, err := s.replayJobRepo.FindByID(ctx, input.WorkspaceID, input.JobID)
	if err != nil {
		if errors.Is(err, domain.ErrReplayJobNotFound) {
			log.Warn("replay job not found")
			return nil, err
		}
		log.Error("failed to get replay job", "error", err)
		return nil, err
	}
	return job, nil
}

type ListReplayJobsInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.ReplayJobFilter
}

func (s *Service) ListReplayJobs(ctx context.Context, input ListReplayJobsInput) ([]domain.ReplayJob, string, error) {
	log := s.log.With("usecase", "list_replay_jobs", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if input.Filter.Status != "" && !domain.IsValidReplayStatus(domain.ReplayJobStatus(input.Filter.Status)) {
		return nil, "", domain.ErrFilterInvalid
	}
	if err := validateReplayJobFilter(input.Filter); err != nil {
		log.Warn("invalid replay job filter", "error", err)
		return nil, "", err
	}
	jobs, cursor, err := s.replayJobRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list replay jobs", "error", err)
		return nil, "", err
	}
	return jobs, cursor, nil
}

func validateOutboxFilter(f domain.OutboxFilter) error {
	if f.Limit < 0 || f.Limit > 100 {
		return domain.ErrFilterInvalid
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return domain.ErrFilterInvalid
	}
	return nil
}

func validateDeadLetterFilter(f domain.DeadLetterFilter) error {
	if f.Limit < 0 || f.Limit > 100 {
		return domain.ErrFilterInvalid
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return domain.ErrFilterInvalid
	}
	return nil
}

func validateReplayJobFilter(f domain.ReplayJobFilter) error {
	if f.Limit < 0 || f.Limit > 100 {
		return domain.ErrFilterInvalid
	}
	return nil
}
