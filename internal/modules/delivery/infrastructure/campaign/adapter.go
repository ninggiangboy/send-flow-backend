package campaign

import (
	"context"
	"encoding/json"

	campaignports "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
)

type CandidateReader struct {
	repo campaignports.CampaignReadRepository
}

func NewCandidateReader(repo campaignports.CampaignReadRepository) *CandidateReader {
	return &CandidateReader{repo: repo}
}

func (r *CandidateReader) ListCandidates(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
	query := campaignports.CandidateListQuery{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		Limit:       limit,
		Cursor:      cursor,
	}

	candidates, nextCursor, err := r.repo.ListCandidates(ctx, query)
	if err != nil {
		return nil, "", err
	}

	result := make([]ports.CampaignCandidate, len(candidates))
	for i, cand := range candidates {
		snapshotRaw, err := json.Marshal(cand.RecipientSnapshot)
		if err != nil {
			return nil, "", err
		}

		result[i] = ports.CampaignCandidate{
			ID:                cand.ID,
			WorkspaceID:       cand.WorkspaceID,
			CampaignID:        cand.CampaignID,
			ContactID:         cand.ContactID,
			EmailNormalized:   cand.EmailNormalized,
			RecipientSnapshot: json.RawMessage(snapshotRaw),
			Status:            string(cand.Status),
		}
	}

	return result, nextCursor, nil
}

func (r *CandidateReader) CountCandidates(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	return r.repo.CountCandidates(ctx, workspaceID, campaignID)
}

var _ ports.CampaignCandidateReader = (*CandidateReader)(nil)
