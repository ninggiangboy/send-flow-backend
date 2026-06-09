package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrUnsupportedEventType = errors.New("unsupported event type")
	ErrInvalidPayload       = errors.New("invalid payload")
)

func ParseUserRegisteredPayload(eventType string, payload []byte) (*UserRegisteredPayload, error) {
	if eventType != EventUserRegisteredV1 {
		return nil, fmt.Errorf("expected %q, got %q: %w", EventUserRegisteredV1, eventType, ErrUnsupportedEventType)
	}
	var p UserRegisteredPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	if p.UserID == "" || p.Email == "" {
		return nil, fmt.Errorf("user_id or email: %w", ErrInvalidPayload)
	}
	return &p, nil
}

func ParseExternalAccountLinkedPayload(eventType string, payload []byte) (*ExternalAccountLinkedPayload, error) {
	if eventType != EventExternalAccountLinkedV1 {
		return nil, fmt.Errorf("expected %q, got %q: %w", EventExternalAccountLinkedV1, eventType, ErrUnsupportedEventType)
	}
	var p ExternalAccountLinkedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	if p.AccountID == "" || p.UserID == "" {
		return nil, fmt.Errorf("account_id or user_id: %w", ErrInvalidPayload)
	}
	return &p, nil
}

func ParseWorkspaceCreatedPayload(eventType string, payload []byte) (*WorkspaceCreatedPayload, error) {
	if eventType != EventWorkspaceCreatedV1 {
		return nil, fmt.Errorf("expected %q, got %q: %w", EventWorkspaceCreatedV1, eventType, ErrUnsupportedEventType)
	}
	var p WorkspaceCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	if p.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id: %w", ErrInvalidPayload)
	}
	return &p, nil
}

func ParseWorkspaceMemberInvitedPayload(eventType string, payload []byte) (*WorkspaceMemberInvitedPayload, error) {
	if eventType != EventWorkspaceMemberInvitedV1 {
		return nil, fmt.Errorf("expected %q, got %q: %w", EventWorkspaceMemberInvitedV1, eventType, ErrUnsupportedEventType)
	}
	var p WorkspaceMemberInvitedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	if p.WorkspaceID == "" || p.Email == "" {
		return nil, fmt.Errorf("workspace_id or email: %w", ErrInvalidPayload)
	}
	return &p, nil
}

func ParseWorkspaceMemberJoinedPayload(eventType string, payload []byte) (*WorkspaceMemberJoinedPayload, error) {
	if eventType != EventWorkspaceMemberJoinedV1 {
		return nil, fmt.Errorf("expected %q, got %q: %w", EventWorkspaceMemberJoinedV1, eventType, ErrUnsupportedEventType)
	}
	var p WorkspaceMemberJoinedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	if p.WorkspaceID == "" || p.UserID == "" {
		return nil, fmt.Errorf("workspace_id or user_id: %w", ErrInvalidPayload)
	}
	return &p, nil
}
