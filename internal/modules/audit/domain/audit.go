package domain

import (
	"fmt"
	"strings"
	"time"
)

type AuditEntry struct {
	ID             string
	WorkspaceID    string
	ActorUserID    string
	ActionType     string
	TargetType     string
	TargetID       string
	PayloadSummary map[string]any
	RequestID      string
	OccurredAt     time.Time
}

func NewAuditEntry(id, workspaceID, actorUserID, actionType, targetType, targetID, requestID string, payloadSummary map[string]any, occurredAt time.Time) (*AuditEntry, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrAuditEntryInvalid)
	}
	if strings.TrimSpace(actionType) == "" {
		return nil, fmt.Errorf("%w: action_type is required", ErrAuditEntryInvalid)
	}
	if len(actionType) > 100 {
		return nil, fmt.Errorf("%w: action_type too long", ErrAuditEntryInvalid)
	}
	if len(targetType) > 100 {
		return nil, fmt.Errorf("%w: target_type too long", ErrAuditEntryInvalid)
	}
	if len(targetID) > 255 {
		return nil, fmt.Errorf("%w: target_id too long", ErrAuditEntryInvalid)
	}
	if len(requestID) > 255 {
		return nil, fmt.Errorf("%w: request_id too long", ErrAuditEntryInvalid)
	}
	return &AuditEntry{
		ID:             id,
		WorkspaceID:    workspaceID,
		ActorUserID:    actorUserID,
		ActionType:     actionType,
		TargetType:     targetType,
		TargetID:       targetID,
		PayloadSummary: payloadSummary,
		RequestID:      requestID,
		OccurredAt:     occurredAt,
	}, nil
}

type AuditFilter struct {
	WorkspaceID string
	ActorUserID string
	ActionType  string
	TargetType  string
	TargetID    string
	From        *time.Time
	To          *time.Time
	Limit       int
	Cursor      string
}
