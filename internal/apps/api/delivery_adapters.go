package api

import (
	"context"
	"errors"

	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	contentdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	deliverydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
)

type transactionalContentRenderer struct {
	svc *contentapp.Service
}

func newTransactionalContentRenderer(svc *contentapp.Service) *transactionalContentRenderer {
	return &transactionalContentRenderer{svc: svc}
}

func (a *transactionalContentRenderer) RenderForMessage(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*deliveryports.RenderedMessage, error) {
	versionID := templateVersionID
	if versionID == "" {
		version, err := a.svc.GetPublishedTemplateVersion(ctx, workspaceID, templateID)
		if err != nil {
			if errors.Is(err, contentdomain.ErrTemplateNotFound) || errors.Is(err, contentdomain.ErrTemplateVersionNotFound) {
				return nil, deliverydomain.ErrTemplateNotFound
			}
			return nil, err
		}
		if version == nil {
			return nil, deliverydomain.ErrTemplateNotFound
		}
		versionID = version.ID
	}

	result, err := a.svc.RenderVersion(ctx, workspaceID, versionID, data)
	if err != nil {
		if errors.Is(err, contentdomain.ErrTemplateNotFound) || errors.Is(err, contentdomain.ErrTemplateVersionNotFound) {
			return nil, deliverydomain.ErrTemplateNotFound
		}
		if errors.Is(err, contentdomain.ErrRenderPayloadInvalid) {
			return nil, deliverydomain.ErrTemplateRenderPayloadInvalid
		}
		return nil, err
	}

	return &deliveryports.RenderedMessage{
		Subject:  result.Result.Subject,
		HTMLBody: result.Result.HTML,
		TextBody: result.Result.Text,
	}, nil
}

type transactionalSenderChecker struct {
	svc *senderapp.Service
}

func newTransactionalSenderChecker(svc *senderapp.Service) *transactionalSenderChecker {
	return &transactionalSenderChecker{svc: svc}
}

func (a *transactionalSenderChecker) GetSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) (*deliveryports.SenderReadiness, error) {
	result, err := a.svc.GetSenderDomainReadiness(ctx, workspaceID, senderDomainID)
	if err != nil {
		if errors.Is(err, senderdomain.ErrDomainNotFound) {
			return nil, deliverydomain.ErrSenderDomainNotFound
		}
		return nil, err
	}
	if result == nil {
		return &deliveryports.SenderReadiness{
			Ready:      false,
			DomainID:   senderDomainID,
			VerifiedAt: "",
		}, nil
	}
	return &deliveryports.SenderReadiness{
		Ready:      result.Ready,
		DomainID:   senderDomainID,
		VerifiedAt: result.CheckedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

type transactionalSuppressionChecker struct {
	svc *suppressionapp.Service
}

func newTransactionalSuppressionChecker(svc *suppressionapp.Service) *transactionalSuppressionChecker {
	return &transactionalSuppressionChecker{svc: svc}
}

func (a *transactionalSuppressionChecker) CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*deliveryports.SuppressionDecision, error) {
	result, err := a.svc.CheckSuppression(ctx, workspaceID, emailNormalized, scope)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &deliveryports.SuppressionDecision{Suppressed: false}, nil
	}
	return &deliveryports.SuppressionDecision{
		Suppressed: result.Suppressed,
		Reason:     result.Reason,
		Scope:      result.Scope,
	}, nil
}
