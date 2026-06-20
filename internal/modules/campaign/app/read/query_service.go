package read

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	CampaignsRead ports.CampaignReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type GetInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
}

type ListInput struct {
	WorkspaceID    string
	Status         string
	SenderDomainID string
	TemplateID     string
	From           string
	To             string
	Limit          int
	Cursor         string
}

type CandidateListInput struct {
	WorkspaceID string
	CampaignID  string
	Status      string
	Limit       int
	Cursor      string
}

type ListResult struct {
	Campaigns  []domain.Campaign
	NextCursor string
}

type CandidateListResult struct {
	Candidates []domain.CampaignMessageCandidate
	NextCursor string
}

type QueryService struct {
	campaignsRead ports.CampaignReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewQueryService(opts Options) *QueryService {
	return &QueryService{
		campaignsRead: opts.CampaignsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("module", "campaign_query"),
	}
}

func (s *QueryService) GetCampaign(ctx context.Context, input GetInput) (*domain.Campaign, int64, error) {
	log := s.log.With("workspace_id", input.WorkspaceID, "campaign_id", input.CampaignID)

	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, 0, err
		}
		return nil, 0, err
	}

	campaign, err := s.campaignsRead.FindByID(ctx, input.WorkspaceID, input.CampaignID)
	if err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, 0, err
		}
		log.Error("failed to find campaign", "error", err)
		return nil, 0, err
	}

	count, err := s.campaignsRead.CountCandidates(ctx, input.WorkspaceID, input.CampaignID)
	if err != nil {
		log.Error("failed to count candidates", "error", err)
	}

	return campaign, count, nil
}

func (s *QueryService) ListCampaigns(ctx context.Context, input ListInput, userID string) ([]domain.Campaign, string, error) {
	log := s.log.With("workspace_id", input.WorkspaceID)

	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, userID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, "", err
		}
		return nil, "", err
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.CampaignListQuery{
		WorkspaceID:    input.WorkspaceID,
		Status:         input.Status,
		SenderDomainID: input.SenderDomainID,
		TemplateID:     input.TemplateID,
		From:           input.From,
		To:             input.To,
		Limit:          limit,
		Cursor:         input.Cursor,
	}

	campaigns, cursor, err := s.campaignsRead.List(ctx, query)
	if err != nil {
		log.Error("failed to list campaigns", "error", err)
		return nil, "", err
	}

	return campaigns, cursor, nil
}

func (s *QueryService) ListCampaignCandidates(ctx context.Context, input CandidateListInput, userID string) ([]domain.CampaignMessageCandidate, string, error) {
	log := s.log.With("workspace_id", input.WorkspaceID)

	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, userID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, "", err
		}
		return nil, "", err
	}

	if _, err := s.campaignsRead.FindByID(ctx, input.WorkspaceID, input.CampaignID); err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, "", err
		}
		log.Error("failed to verify campaign", "error", err)
		return nil, "", err
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.CandidateListQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		Status:      input.Status,
		Limit:       limit,
		Cursor:      input.Cursor,
	}

	candidates, cursor, err := s.campaignsRead.ListCandidates(ctx, query)
	if err != nil {
		log.Error("failed to list candidates", "error", err)
		return nil, "", err
	}

	return candidates, cursor, nil
}
