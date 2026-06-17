package worker

import (
	"context"
	"io"
	"time"

	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliverydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/objectstorage"
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

type deliveryObjectStorageAdapter struct {
	client objectstorage.ObjectStorage
}

func (a deliveryObjectStorageAdapter) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	if a.client == nil {
		return deliverydomain.ErrObjectStorageDisabled
	}
	return a.client.PutObject(ctx, key, body, contentType)
}

func (a deliveryObjectStorageAdapter) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	if a.client == nil {
		return nil, deliverydomain.ErrObjectStorageDisabled
	}
	return a.client.GetObject(ctx, key)
}

func (a deliveryObjectStorageAdapter) DeleteObject(ctx context.Context, key string) error {
	if a.client == nil {
		return deliverydomain.ErrObjectStorageDisabled
	}
	return a.client.DeleteObject(ctx, key)
}

var (
	_ deliveryports.SuppressionChecker     = (*suppressionAdapter)(nil)
	_ deliveryports.ContentRenderer        = (*contentRendererAdapter)(nil)
	_ deliveryports.SenderReadinessChecker = (*senderReadinessAdapter)(nil)
	_ deliveryapp.RecipientSuppressor      = (*recipientSuppressorAdapter)(nil)
	_ deliveryports.ObjectStorage          = deliveryObjectStorageAdapter{}
)
