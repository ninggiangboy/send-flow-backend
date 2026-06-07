package ports

import (
	"context"
	"encoding/json"
)

type CampaignCandidate struct {
	ID                string
	WorkspaceID       string
	CampaignID        string
	ContactID         string
	EmailNormalized   string
	RecipientSnapshot json.RawMessage
	Status            string
}

type CampaignCandidateReader interface {
	ListCandidates(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]CampaignCandidate, string, error)
	CountCandidates(ctx context.Context, workspaceID, campaignID string) (int64, error)
}
