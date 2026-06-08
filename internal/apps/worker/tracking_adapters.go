package worker

import (
	"context"

	deliverypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
)

type trackingMessageResolverAdapter struct {
	msgReadRepo *deliverypostgres.MessageReadRepository
}

func newTrackingMessageResolverAdapter(msgReadRepo *deliverypostgres.MessageReadRepository) *trackingMessageResolverAdapter {
	return &trackingMessageResolverAdapter{msgReadRepo: msgReadRepo}
}

func (a *trackingMessageResolverAdapter) FindByID(ctx context.Context, workspaceID, messageID string) (string, string, string, string, string, string, error) {
	msg, err := a.msgReadRepo.FindByID(ctx, workspaceID, messageID)
	if err != nil {
		return "", "", "", "", "", "", err
	}
	if msg == nil {
		return "", "", "", "", "", "", nil
	}
	return msg.ID, msg.WorkspaceID, msg.CampaignID, msg.Provider, msg.ProviderMessageID, msg.RecipientEmailNormalized, nil
}

func (a *trackingMessageResolverAdapter) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (string, string, string, error) {
	msg, err := a.msgReadRepo.FindByProviderMessageID(ctx, provider, providerMessageID)
	if err != nil {
		return "", "", "", err
	}
	if msg == nil {
		return "", "", "", nil
	}
	return msg.ID, msg.WorkspaceID, msg.CampaignID, nil
}

type trackingSuppressorAdapter struct {
	svc *suppressionapp.Service
}

func newTrackingSuppressorAdapter(svc *suppressionapp.Service) *trackingSuppressorAdapter {
	return &trackingSuppressorAdapter{svc: svc}
}

func (a *trackingSuppressorAdapter) SuppressFromSignal(ctx context.Context, input trackingapp.SuppressFromSignalInput) (*trackingapp.SuppressFromSignalResult, error) {
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
