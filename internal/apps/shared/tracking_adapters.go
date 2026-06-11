package shared

import (
	"context"

	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
)

type TrackingMessageResolverAdapter struct {
	resolver deliveryports.MessageResolver
}

func NewTrackingMessageResolverAdapter(resolver deliveryports.MessageResolver) *TrackingMessageResolverAdapter {
	return &TrackingMessageResolverAdapter{resolver: resolver}
}

func (a *TrackingMessageResolverAdapter) FindByID(ctx context.Context, workspaceID, messageID string) (string, string, string, string, string, string, error) {
	summary, err := a.resolver.FindByID(ctx, workspaceID, messageID)
	if err != nil {
		return "", "", "", "", "", "", err
	}
	if summary == nil {
		return "", "", "", "", "", "", nil
	}
	return summary.ID, summary.WorkspaceID, summary.CampaignID, summary.Provider, summary.ProviderMessageID, summary.RecipientEmailNormalized, nil
}

func (a *TrackingMessageResolverAdapter) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (string, string, string, error) {
	summary, err := a.resolver.FindByProviderMessageID(ctx, provider, providerMessageID)
	if err != nil {
		return "", "", "", err
	}
	if summary == nil {
		return "", "", "", nil
	}
	return summary.ID, summary.WorkspaceID, summary.CampaignID, nil
}

type TrackingSuppressorAdapter struct {
	svc *suppressionapp.Service
}

func NewTrackingSuppressorAdapter(svc *suppressionapp.Service) *TrackingSuppressorAdapter {
	return &TrackingSuppressorAdapter{svc: svc}
}

func (a *TrackingSuppressorAdapter) SuppressFromSignal(ctx context.Context, input trackingapp.SuppressFromSignalInput) (*trackingapp.SuppressFromSignalResult, error) {
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
	return &trackingapp.SuppressFromSignalResult{EntryID: entry.ID, Created: created}, nil
}
