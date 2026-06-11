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

type OutboxEventResult struct {
	ID            string          `json:"id"`
	WorkspaceID   string          `json:"workspace_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	Headers       json.RawMessage `json:"headers"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

type DeadLetterResult struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspace_id"`
	Source       string          `json:"source"`
	EventID      string          `json:"event_id"`
	Payload      json.RawMessage `json:"payload"`
	ErrorMessage string          `json:"error_message"`
	Retryable    bool            `json:"retryable"`
	FailedAt     time.Time       `json:"failed_at"`
}

type ReplayJobResult struct {
	ID                string                  `json:"id"`
	WorkspaceID       string                  `json:"workspace_id"`
	TargetType        domain.ReplayTargetType `json:"target_type"`
	TargetID          string                  `json:"target_id"`
	Source            string                  `json:"source"`
	Status            domain.ReplayJobStatus  `json:"status"`
	RequestedByUserID string                  `json:"requested_by_user_id"`
	Reason            string                  `json:"reason"`
	Filter            json.RawMessage         `json:"filter"`
	Result            json.RawMessage         `json:"result"`
	ErrorMessage      string                  `json:"error_message"`
	CreatedAt         time.Time               `json:"created_at"`
	StartedAt         *time.Time              `json:"started_at"`
	CompletedAt       *time.Time              `json:"completed_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type OutboxSummaryResult struct {
	TotalCount   int            `json:"total_count"`
	OldestAgeSec int64          `json:"oldest_age_seconds"`
	OldestAt     *time.Time     `json:"oldest_at,omitempty"`
	ByEventType  map[string]int `json:"by_event_type,omitempty"`
}

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

// --- Mappers ---

func outboxRecordToResult(r domain.OutboxRecord) OutboxEventResult {
	return OutboxEventResult{
		ID:            r.ID,
		WorkspaceID:   r.WorkspaceID,
		AggregateType: r.AggregateType,
		AggregateID:   r.AggregateID,
		EventType:     r.EventType,
		Payload:       r.Payload,
		Headers:       r.Headers,
		OccurredAt:    r.OccurredAt,
		CreatedAt:     r.CreatedAt,
	}
}

func deadLetterToResult(r domain.DeadLetterRecord) DeadLetterResult {
	return DeadLetterResult{
		ID:           r.ID,
		WorkspaceID:  r.WorkspaceID,
		Source:       r.Source,
		EventID:      r.EventID,
		Payload:      r.Payload,
		ErrorMessage: r.ErrorMessage,
		Retryable:    r.Retryable,
		FailedAt:     r.FailedAt,
	}
}

func replayJobToResult(j domain.ReplayJob) ReplayJobResult {
	return ReplayJobResult{
		ID:                j.ID,
		WorkspaceID:       j.WorkspaceID,
		TargetType:        j.TargetType,
		TargetID:          j.TargetID,
		Source:            j.Source,
		Status:            j.Status,
		RequestedByUserID: j.RequestedByUserID,
		Reason:            j.Reason,
		Filter:            j.Filter,
		Result:            j.Result,
		ErrorMessage:      j.ErrorMessage,
		CreatedAt:         j.CreatedAt,
		StartedAt:         j.StartedAt,
		CompletedAt:       j.CompletedAt,
		UpdatedAt:         j.UpdatedAt,
	}
}

func outboxSummaryToResult(s domain.OutboxSummary) OutboxSummaryResult {
	return OutboxSummaryResult{
		TotalCount:   s.TotalCount,
		OldestAgeSec: s.OldestAgeSec,
		OldestAt:     s.OldestAt,
		ByEventType:  s.ByEventType,
	}
}

// --- Inputs ---

type GetOutboxSummaryInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.OutboxFilter
}

func (s *Service) GetOutboxSummary(ctx context.Context, input GetOutboxSummaryInput) (OutboxSummaryResult, error) {
	log := s.log.With("usecase", "get_outbox_summary", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		if errors.Is(err, domain.ErrQueueReadDenied) || errors.Is(err, domain.ErrDLQReadDenied) || errors.Is(err, domain.ErrReplayManageDenied) {
			return OutboxSummaryResult{}, err
		}
		return OutboxSummaryResult{}, err
	}
	if input.WorkspaceID == "" {
		log.Warn("missing workspace id")
		return OutboxSummaryResult{}, domain.ErrWorkspaceRequired
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid outbox filter", "error", err)
		return OutboxSummaryResult{}, err
	}
	summary, err := s.outboxRepo.GetSummary(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to get outbox summary", "error", err)
		return OutboxSummaryResult{}, err
	}
	return outboxSummaryToResult(summary), nil
}

type ListOutboxRecordsInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.OutboxFilter
}

func (s *Service) ListOutboxRecords(ctx context.Context, input ListOutboxRecordsInput) ([]OutboxEventResult, string, error) {
	log := s.log.With("usecase", "list_outbox_records", "workspace_id", input.WorkspaceID)
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
	results := make([]OutboxEventResult, len(records))
	for i := range records {
		results[i] = outboxRecordToResult(records[i])
	}
	return results, cursor, nil
}

type GetOutboxRecordInput struct {
	WorkspaceID string
	UserID      string
	OutboxID    string
}

func (s *Service) GetOutboxRecord(ctx context.Context, input GetOutboxRecordInput) (*OutboxEventResult, error) {
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
	result := outboxRecordToResult(*rec)
	return &result, nil
}

type ListDeadLetterRecordsInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.DeadLetterFilter
}

func (s *Service) ListDeadLetterRecords(ctx context.Context, input ListDeadLetterRecordsInput) ([]DeadLetterResult, string, error) {
	log := s.log.With("usecase", "list_dead_letter_records", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.dlq.read"); err != nil {
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
	results := make([]DeadLetterResult, len(records))
	for i := range records {
		results[i] = deadLetterToResult(records[i])
	}
	return results, cursor, nil
}

type GetDeadLetterRecordInput struct {
	WorkspaceID string
	UserID      string
	RecordID    string
}

func (s *Service) GetDeadLetterRecord(ctx context.Context, input GetDeadLetterRecordInput) (*DeadLetterResult, error) {
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
	result := deadLetterToResult(*rec)
	return &result, nil
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

func (s *Service) CreateReplayJob(ctx context.Context, input CreateReplayJobInput) (*ReplayJobResult, error) {
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
	result := replayJobToResult(job)
	return &result, nil
}

type RunReplayJobInput struct {
	WorkspaceID string
	UserID      string
	JobID       string
}

func (s *Service) RunReplayJob(ctx context.Context, input RunReplayJobInput) (*ReplayJobResult, error) {
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
			result := replayJobToResult(*job)
			return &result, nil
		}
		return nil, resultErr
	}

	resultData := map[string]any{"replayed": true}
	if err := s.replayJobRepo.MarkCompleted(ctx, input.WorkspaceID, input.JobID, resultData, now); err != nil {
		log.Error("failed to mark job completed", "error", err)
		return nil, err
	}
	job.Status = domain.ReplayJobCompleted
	job.CompletedAt = &now
	job.Result, _ = json.Marshal(resultData)

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
	result := replayJobToResult(*job)
	return &result, nil
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

func (s *Service) GetReplayJob(ctx context.Context, input GetReplayJobInput) (*ReplayJobResult, error) {
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
	result := replayJobToResult(*job)
	return &result, nil
}

type ListReplayJobsInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.ReplayJobFilter
}

func (s *Service) ListReplayJobs(ctx context.Context, input ListReplayJobsInput) ([]ReplayJobResult, string, error) {
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
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid replay job filter", "error", err)
		return nil, "", err
	}
	jobs, cursor, err := s.replayJobRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list replay jobs", "error", err)
		return nil, "", err
	}
	results := make([]ReplayJobResult, len(jobs))
	for i := range jobs {
		results[i] = replayJobToResult(jobs[i])
	}
	return results, cursor, nil
}
