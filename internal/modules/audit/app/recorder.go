package app

import (
	"context"
	"time"
)

type AuditRecorder interface {
	Record(ctx context.Context, input RecordAuditEntryInput) error
}

type RecordAuditEntryInput struct {
	WorkspaceID    string
	ActorUserID    string
	ActionType     string
	TargetType     string
	TargetID       string
	PayloadSummary map[string]any
	RequestID      string
	OccurredAt     time.Time
}
