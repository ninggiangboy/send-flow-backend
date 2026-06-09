package api

import (
	"context"

	auditapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
)

type permissionCheckerAdapter struct {
	svc *identityapp.Service
}

func newPermissionCheckerAdapter(svc *identityapp.Service) *permissionCheckerAdapter {
	return &permissionCheckerAdapter{svc: svc}
}

func (a *permissionCheckerAdapter) RequireWorkspacePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return a.svc.RequireWorkspacePermission(ctx, workspaceID, userID, permission)
}

type auditRecorderAdapter struct {
	svc *auditapp.Service
}

func newAuditRecorderAdapter(svc *auditapp.Service) *auditRecorderAdapter {
	return &auditRecorderAdapter{svc: svc}
}

func (a *auditRecorderAdapter) Record(ctx context.Context, input identityapp.RecordAuditInput) error {
	return a.svc.RecordAuditEntry(ctx, auditapp.RecordAuditEntryInput{
		WorkspaceID:    input.WorkspaceID,
		ActorUserID:    input.ActorUserID,
		ActionType:     input.ActionType,
		TargetType:     input.TargetType,
		TargetID:       input.TargetID,
		PayloadSummary: input.PayloadSummary,
		RequestID:      input.RequestID,
		OccurredAt:     input.OccurredAt,
	})
}
