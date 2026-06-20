package app

import (
	"encoding/json"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/deadletter"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/replay"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

// --- Input Type Aliases (for API layer backward compat) ---
type GetOutboxSummaryInput = outbox.SummaryInput
type ListOutboxRecordsInput = outbox.ListInput
type GetOutboxRecordInput = outbox.GetInput
type ListDeadLetterRecordsInput = deadletter.ListInput
type GetDeadLetterRecordInput = deadletter.GetInput
type CreateReplayJobInput = replay.CreateInput
type RunReplayJobInput = replay.RunInput
type GetReplayJobInput = replay.GetInput
type ListReplayJobsInput = replay.ListInput

// --- Shared Result Types ---

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

// --- Exported Mappers ---

func OutboxRecordToResult(r domain.OutboxRecord) OutboxEventResult {
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

func DeadLetterToResult(r domain.DeadLetterRecord) DeadLetterResult {
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

func ReplayJobToResult(j domain.ReplayJob) ReplayJobResult {
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

func OutboxSummaryToResult(s domain.OutboxSummary) OutboxSummaryResult {
	return OutboxSummaryResult{
		TotalCount:   s.TotalCount,
		OldestAgeSec: s.OldestAgeSec,
		OldestAt:     s.OldestAt,
		ByEventType:  s.ByEventType,
	}
}
