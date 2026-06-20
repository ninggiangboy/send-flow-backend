package app

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/draft"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/read"
)

type CommandBus interface {
	CreateCampaign(ctx context.Context, input CreateCampaignInput) (*CampaignResult, error)
	UpdateCampaignDraft(ctx context.Context, input UpdateCampaignDraftInput) (*CampaignResult, error)
	ScheduleCampaign(ctx context.Context, input ScheduleCampaignInput) (*CampaignResult, error)
	CancelCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error)
	PauseCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error)
	ResumeCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error)
}

type QueryBus interface {
	ListCampaigns(ctx context.Context, input ListInput, userID string) (*CampaignListResult, error)
	GetCampaign(ctx context.Context, workspaceID, campaignID, userID string) (*CampaignResult, error)
	ListCampaignCandidates(ctx context.Context, input CandidateListInput, userID string) (*CandidateListResult, error)
}

type commandBus struct {
	create   *draft.CreateHandler
	update   *draft.UpdateHandler
	schedule *draft.ScheduleHandler
	cancel   *draft.CancelHandler
	pause    *draft.PauseHandler
	resume   *draft.ResumeHandler
}

func newCommandBus(
	createH *draft.CreateHandler,
	updateH *draft.UpdateHandler,
	scheduleH *draft.ScheduleHandler,
	cancelH *draft.CancelHandler,
	pauseH *draft.PauseHandler,
	resumeH *draft.ResumeHandler,
) CommandBus {
	return &commandBus{
		create:   createH,
		update:   updateH,
		schedule: scheduleH,
		cancel:   cancelH,
		pause:    pauseH,
		resume:   resumeH,
	}
}

func (b *commandBus) CreateCampaign(ctx context.Context, input CreateCampaignInput) (*CampaignResult, error) {
	result, err := b.create.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *result}, nil
}

func (b *commandBus) UpdateCampaignDraft(ctx context.Context, input UpdateCampaignDraftInput) (*CampaignResult, error) {
	result, err := b.update.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *result}, nil
}

func (b *commandBus) ScheduleCampaign(ctx context.Context, input ScheduleCampaignInput) (*CampaignResult, error) {
	result, err := b.schedule.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	return &CampaignResult{
		Campaign:       *result.Campaign,
		CandidateCount: int64(len(result.Candidates)),
	}, nil
}

func (b *commandBus) CancelCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	result, err := b.cancel.Execute(ctx, draft.CancelInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *result}, nil
}

func (b *commandBus) PauseCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	result, err := b.pause.Execute(ctx, draft.PauseInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *result}, nil
}

func (b *commandBus) ResumeCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	result, err := b.resume.Execute(ctx, draft.ResumeInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *result}, nil
}

type queryBus struct {
	queryService *read.QueryService
}

func newQueryBus(queryService *read.QueryService) QueryBus {
	return &queryBus{queryService: queryService}
}

func (b *queryBus) ListCampaigns(ctx context.Context, input ListInput, userID string) (*CampaignListResult, error) {
	ri := read.ListInput{
		WorkspaceID:    input.WorkspaceID,
		Status:         input.Status,
		SenderDomainID: input.SenderDomainID,
		TemplateID:     input.TemplateID,
		From:           input.From,
		To:             input.To,
		Limit:          input.Limit,
		Cursor:         input.Cursor,
	}
	campaigns, cursor, err := b.queryService.ListCampaigns(ctx, ri, userID)
	if err != nil {
		return nil, err
	}
	return &CampaignListResult{Campaigns: campaigns, NextCursor: cursor}, nil
}

func (b *queryBus) GetCampaign(ctx context.Context, workspaceID, campaignID, userID string) (*CampaignResult, error) {
	campaign, count, err := b.queryService.GetCampaign(ctx, read.GetInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *campaign, CandidateCount: count}, nil
}

func (b *queryBus) ListCampaignCandidates(ctx context.Context, input CandidateListInput, userID string) (*CandidateListResult, error) {
	ri := read.CandidateListInput{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		Status:      input.Status,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	}
	candidates, cursor, err := b.queryService.ListCampaignCandidates(ctx, ri, userID)
	if err != nil {
		return nil, err
	}
	return &CandidateListResult{Candidates: candidates, NextCursor: cursor}, nil
}
