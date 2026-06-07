package domain

import (
	"strings"
	"time"
)

type SuppressionStatus string

const (
	SuppressionStatusActive  SuppressionStatus = "active"
	SuppressionStatusRemoved SuppressionStatus = "removed"
)

type SuppressionScope string

const (
	SuppressionScopeWorkspace SuppressionScope = "workspace"
	SuppressionScopeList      SuppressionScope = "list"
	SuppressionScopeGlobal    SuppressionScope = "global"
)

type SuppressionReason string

const (
	SuppressionReasonManualBlock SuppressionReason = "manual_block"
	SuppressionReasonBounce      SuppressionReason = "bounce"
	SuppressionReasonComplaint   SuppressionReason = "complaint"
	SuppressionReasonUnsubscribe SuppressionReason = "unsubscribe"
)

type SuppressionEntry struct {
	ID              string
	WorkspaceID     string
	Email           string
	EmailNormalized string
	Scope           SuppressionScope
	Reason          SuppressionReason
	Status          SuppressionStatus
	Note            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	RemovedAt       *time.Time
}

func ValidSuppressionScope(s string) bool {
	switch SuppressionScope(s) {
	case SuppressionScopeWorkspace, SuppressionScopeList, SuppressionScopeGlobal:
		return true
	default:
		return false
	}
}

func ValidSuppressionReason(s string) bool {
	switch SuppressionReason(s) {
	case SuppressionReasonManualBlock, SuppressionReasonBounce, SuppressionReasonComplaint, SuppressionReasonUnsubscribe:
		return true
	default:
		return false
	}
}

func NormalizeEmail(email string) string {
	trimmed := strings.TrimSpace(email)
	return strings.ToLower(trimmed)
}
