package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
)

type CampaignListQuery struct {
	WorkspaceID    string
	Status         string
	SenderDomainID string
	TemplateID     string
	From           string
	To             string
	Limit          int
	Cursor         string
}

type CandidateListQuery struct {
	WorkspaceID string
	CampaignID  string
	Status      string
	Limit       int
	Cursor      string
}

type CampaignReadRepository interface {
	FindByID(ctx context.Context, workspaceID, campaignID string) (*domain.Campaign, error)
	List(ctx context.Context, query CampaignListQuery) ([]domain.Campaign, string, error)
	ListCandidates(ctx context.Context, query CandidateListQuery) ([]domain.CampaignMessageCandidate, string, error)
	CountCandidates(ctx context.Context, workspaceID, campaignID string) (int64, error)
}

type CampaignWriteRepository interface {
	CampaignReadRepository
	Create(ctx context.Context, campaign domain.Campaign) error
	Update(ctx context.Context, campaign domain.Campaign) error
	ReplaceCandidates(ctx context.Context, workspaceID, campaignID string, candidates []domain.CampaignMessageCandidate) error
}
