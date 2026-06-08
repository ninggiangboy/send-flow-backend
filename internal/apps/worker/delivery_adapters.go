package worker

import (
	"context"
	"time"

	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
)

type suppressionAdapter struct {
	svc *suppressionapp.Service
}

func newSuppressionAdapter(svc *suppressionapp.Service) *suppressionAdapter {
	return &suppressionAdapter{svc: svc}
}

func (a *suppressionAdapter) CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*deliveryports.SuppressionDecision, error) {
	result, err := a.svc.CheckSuppression(ctx, workspaceID, emailNormalized, scope)
	if err != nil {
		return nil, err
	}
	return &deliveryports.SuppressionDecision{
		Suppressed: result.Suppressed,
		Reason:     result.Reason,
		Scope:      result.Scope,
	}, nil
}

type contentRendererAdapter struct {
	svc *contentapp.Service
}

func newContentRendererAdapter(svc *contentapp.Service) *contentRendererAdapter {
	return &contentRendererAdapter{svc: svc}
}

func (a *contentRendererAdapter) RenderForMessage(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*deliveryports.RenderedMessage, error) {
	result, err := a.svc.RenderVersion(ctx, workspaceID, templateVersionID, data)
	if err != nil {
		return nil, err
	}
	return &deliveryports.RenderedMessage{
		Subject:  result.Result.Subject,
		HTMLBody: result.Result.HTML,
		TextBody: result.Result.Text,
	}, nil
}

type senderReadinessAdapter struct {
	svc *senderapp.Service
}

func newSenderReadinessAdapter(svc *senderapp.Service) *senderReadinessAdapter {
	return &senderReadinessAdapter{svc: svc}
}

func (a *senderReadinessAdapter) GetSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) (*deliveryports.SenderReadiness, error) {
	result, err := a.svc.GetSenderDomainReadiness(ctx, workspaceID, senderDomainID)
	if err != nil {
		return nil, err
	}
	return &deliveryports.SenderReadiness{
		Ready:      result.Ready,
		DomainID:   senderDomainID,
		VerifiedAt: result.CheckedAt.Format(time.RFC3339),
	}, nil
}

type recipientSuppressorAdapter struct {
	svc *suppressionapp.Service
}

func newRecipientSuppressorAdapter(svc *suppressionapp.Service) *recipientSuppressorAdapter {
	return &recipientSuppressorAdapter{svc: svc}
}

func (a *recipientSuppressorAdapter) SuppressFromSignal(ctx context.Context, input deliveryapp.SuppressFromSignalInput) (*deliveryapp.SuppressFromSignalResult, error) {
	suppressionInput := suppressionapp.CreateSystemEntryInput{
		WorkspaceID:     input.WorkspaceID,
		Email:           input.Email,
		EmailNormalized: input.EmailNormalized,
		Scope:           input.Scope,
		Reason:          input.Reason,
		Source:          input.Source,
		SourceEventID:   input.SourceEventID,
		Note:            input.Note,
		Now:             input.Now,
	}
	entry, created, err := a.svc.CreateSystemEntry(ctx, suppressionInput)
	if err != nil {
		return nil, err
	}
	return &deliveryapp.SuppressFromSignalResult{EntryID: entry.ID, Created: created}, nil
}

var (
	_ deliveryports.SuppressionChecker     = (*suppressionAdapter)(nil)
	_ deliveryports.ContentRenderer        = (*contentRendererAdapter)(nil)
	_ deliveryports.SenderReadinessChecker = (*senderReadinessAdapter)(nil)
	_ deliveryapp.RecipientSuppressor      = (*recipientSuppressorAdapter)(nil)
)
