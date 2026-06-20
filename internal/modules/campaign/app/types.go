package app

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/draft"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/read"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
)

// Type aliases for backward compatibility with external consumers (API, worker, tests).
// These allow grouped-package types to be accessed via campaignapp.X without import changes.

// Draft commands
type CreateCampaignInput = draft.CreateInput
type UpdateCampaignDraftInput = draft.UpdateInput
type ScheduleCampaignInput = draft.ScheduleInput

// Read queries
type ListInput = read.ListInput
type CandidateListInput = read.CandidateListInput
type CandidateListResult = read.CandidateListResult

// CampaignResult wraps a campaign domain object with an optional candidate count.
// Used by facade methods that combine domain results with derived data.
type CampaignResult struct {
	Campaign       domain.Campaign
	CandidateCount int64
}

// CampaignListResult is kept as a concrete type (not alias) because the facade
// constructs it directly from domain query results.
type CampaignListResult struct {
	Campaigns  []domain.Campaign
	NextCursor string
}

// ScheduleResult is only used internally by the schedule handler and the CQRS layer;
// the facade maps it to CampaignResult.
type ScheduleResult = draft.ScheduleResult
