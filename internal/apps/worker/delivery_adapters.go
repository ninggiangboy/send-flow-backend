package worker

import (
	"context"
	"time"

	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
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

var (
	_ deliveryports.SuppressionChecker     = (*suppressionAdapter)(nil)
	_ deliveryports.ContentRenderer        = (*contentRendererAdapter)(nil)
	_ deliveryports.SenderReadinessChecker = (*senderReadinessAdapter)(nil)
)
