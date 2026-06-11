package domain

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

type OutboxRecord struct {
	ID            string
	WorkspaceID   string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       json.RawMessage
	Headers       json.RawMessage
	OccurredAt    time.Time
	CreatedAt     time.Time
}

type DeadLetterRecord struct {
	ID              string
	WorkspaceID     string
	Source          string
	SourceEventType string
	EventID         string
	Payload         json.RawMessage
	ErrorMessage    string
	Retryable       bool
	FailedAt        time.Time
}

type ReplayTargetType string

const (
	ReplayTargetDeadLetter ReplayTargetType = "dead_letter_record"
	ReplayTargetOutbox     ReplayTargetType = "outbox_event"
)

type ReplayJobStatus string

const (
	ReplayJobQueued    ReplayJobStatus = "queued"
	ReplayJobRunning   ReplayJobStatus = "running"
	ReplayJobCompleted ReplayJobStatus = "completed"
	ReplayJobFailed    ReplayJobStatus = "failed"
)

type ReplayJob struct {
	ID                string
	WorkspaceID       string
	TargetType        ReplayTargetType
	TargetID          string
	Source            string
	Status            ReplayJobStatus
	RequestedByUserID string
	Reason            string
	Filter            json.RawMessage
	Result            json.RawMessage
	ErrorMessage      string
	CreatedAt         time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
	UpdatedAt         time.Time
}

type OutboxFilter struct {
	EventType     string
	AggregateType string
	AggregateID   string
	From          *time.Time
	To            *time.Time
	Limit         int
	Cursor        string
}

func (f OutboxFilter) Validate() error {
	if f.Limit < 0 || f.Limit > 100 {
		return ErrFilterInvalid
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return ErrFilterInvalid
	}
	return nil
}

type DeadLetterFilter struct {
	Source    string
	Retryable *bool
	From      *time.Time
	To        *time.Time
	Limit     int
	Cursor    string
}

func (f DeadLetterFilter) Validate() error {
	if f.Limit < 0 || f.Limit > 100 {
		return ErrFilterInvalid
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return ErrFilterInvalid
	}
	return nil
}

type ReplayJobFilter struct {
	Status string
	Limit  int
	Cursor string
}

func (f ReplayJobFilter) Validate() error {
	if f.Limit < 0 || f.Limit > 100 {
		return ErrFilterInvalid
	}
	return nil
}

type OutboxSummary struct {
	TotalCount   int
	OldestAgeSec int64
	OldestAt     *time.Time
	ByEventType  map[string]int
}

var supportedTargetTypes = []ReplayTargetType{
	ReplayTargetDeadLetter,
	ReplayTargetOutbox,
}

var validReplayStatuses = []ReplayJobStatus{
	ReplayJobQueued,
	ReplayJobRunning,
	ReplayJobCompleted,
	ReplayJobFailed,
}

func IsValidTargetType(t ReplayTargetType) bool {
	return slices.Contains(supportedTargetTypes, t)
}

func IsValidReplayStatus(s ReplayJobStatus) bool {
	return slices.Contains(validReplayStatuses, s)
}

func SanitizeErrorMessage(msg string) string {
	if len(msg) > 2000 {
		return msg[:2000]
	}
	return msg
}

func SanitizePayloadPreview(raw json.RawMessage, maxBytes int) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	if len(raw) <= maxBytes {
		return raw
	}
	truncated := raw[:maxBytes]
	if json.Valid(truncated) {
		return truncated
	}
	return json.RawMessage("{}")
}

var sensitiveKeys = []string{
	"authorization", "cookie", "password", "secret",
	"token", "api_key", "refresh_token", "access_token",
	"auth", "signature",
}

func RedactSensitiveFields(data json.RawMessage) json.RawMessage {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	redactValue(&v)
	result, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return result
}

func redactValue(v *any) {
	switch val := (*v).(type) {
	case map[string]any:
		for k, fieldVal := range val {
			if isSensitiveKey(k) {
				val[k] = "[REDACTED]"
			} else {
				redactValue(&fieldVal)
				val[k] = fieldVal
			}
		}
	case []any:
		for i := range val {
			redactValue(&val[i])
		}
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, sk := range sensitiveKeys {
		if strings.Contains(lower, sk) {
			return true
		}
	}
	return false
}
