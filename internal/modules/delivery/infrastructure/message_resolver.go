package infrastructure

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	ingestionports "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

type messageResolverAdapter struct {
	repo *postgres.MessageReadRepository
}

func NewMessageResolver(repo *postgres.MessageReadRepository) deliveryports.MessageResolver {
	return &messageResolverAdapter{repo: repo}
}

func (a *messageResolverAdapter) FindByID(ctx context.Context, workspaceID, messageID string) (*deliveryports.MessageSummary, error) {
	msg, err := a.repo.FindByID(ctx, workspaceID, messageID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil
	}
	return &deliveryports.MessageSummary{
		ID:                       msg.ID,
		WorkspaceID:              msg.WorkspaceID,
		CampaignID:               msg.CampaignID,
		Provider:                 msg.Provider,
		ProviderMessageID:        msg.ProviderMessageID,
		RecipientEmailNormalized: msg.RecipientEmailNormalized,
	}, nil
}

func (a *messageResolverAdapter) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*deliveryports.MessageSummary, error) {
	msg, err := a.repo.FindByProviderMessageID(ctx, provider, providerMessageID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil
	}
	return &deliveryports.MessageSummary{
		ID:                       msg.ID,
		WorkspaceID:              msg.WorkspaceID,
		CampaignID:               msg.CampaignID,
		Provider:                 msg.Provider,
		ProviderMessageID:        msg.ProviderMessageID,
		RecipientEmailNormalized: msg.RecipientEmailNormalized,
	}, nil
}

type ingestionMessageResolverAdapter struct {
	repo *postgres.MessageReadRepository
}

func NewIngestionMessageResolver(repo *postgres.MessageReadRepository) ingestionports.DeliveryMessageResolver {
	return &ingestionMessageResolverAdapter{repo: repo}
}

func (a *ingestionMessageResolverAdapter) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*ingestionports.MessageRef, error) {
	msg, err := a.repo.FindByProviderMessageID(ctx, provider, providerMessageID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil
	}
	return &ingestionports.MessageRef{
		WorkspaceID:       msg.WorkspaceID,
		MessageID:         msg.ID,
		Provider:          msg.Provider,
		ProviderMessageID: msg.ProviderMessageID,
	}, nil
}
