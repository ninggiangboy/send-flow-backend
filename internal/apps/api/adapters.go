package api

import (
	"context"
	"time"

	auditapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/ratelimit"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
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

type mailerAdapter struct {
	sender email.Sender
}

func (a *mailerAdapter) Send(ctx context.Context, to []string, subject, text, html string) error {
	return a.sender.Send(ctx, email.Message{To: to, Subject: subject, Text: text, HTML: html})
}

type uuidIDGeneratorAdapter struct {
	gen id.UUIDGenerator
}

func (a *uuidIDGeneratorAdapter) New() (string, error) {
	return a.gen.New()
}

type tokenGeneratorAdapter struct{}

func (a *tokenGeneratorAdapter) RandomToken(n int) (string, error) {
	return security.RandomToken(n)
}

type tokenHasherAdapter struct{}

func (a *tokenHasherAdapter) HashToken(token string) string {
	return security.HashToken(token)
}

type passwordValidatorAdapter struct{}

func (a *passwordValidatorAdapter) Validate(password string) error {
	return security.ValidatePasswordPolicy(password)
}

type totpVerifierAdapter struct{}

func (a *totpVerifierAdapter) VerifyTOTPCode(secret, code string, now time.Time) bool {
	return security.VerifyTOTPCode(secret, code, now)
}

type totpSecretGeneratorAdapter struct{}

func (a *totpSecretGeneratorAdapter) GenerateTOTPSecret() (string, error) {
	return security.GenerateTOTPSecret()
}

type recoveryCodeGeneratorAdapter struct{}

func (a *recoveryCodeGeneratorAdapter) GenerateRecoveryCode() (string, error) {
	return security.GenerateRecoveryCode()
}

type rateLimiterAdapter struct {
	svc ratelimit.Service
}

func (a *rateLimiterAdapter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	return a.svc.Allow(ctx, key, limit, window)
}