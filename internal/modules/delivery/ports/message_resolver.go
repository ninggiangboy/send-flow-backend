package ports

import "context"

type MessageSummary struct {
	ID                       string
	WorkspaceID              string
	CampaignID               string
	Provider                 string
	ProviderMessageID        string
	RecipientEmailNormalized string
}

type MessageResolver interface {
	FindByID(ctx context.Context, workspaceID, messageID string) (*MessageSummary, error)
	FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*MessageSummary, error)
}
