package app

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/cancelcampaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/createcampaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/getcampaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/listcampaigncandidates"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/listcampaigns"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/pausecampaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/resumecampaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/schedulecampaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/updatecampaigndraft"
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
	create   *createcampaign.Handler
	update   *updatecampaigndraft.Handler
	schedule *schedulecampaign.Handler
	cancel   *cancelcampaign.Handler
	pause    *pausecampaign.Handler
	resume   *resumecampaign.Handler
}

func newCommandBus(
	createH *createcampaign.Handler,
	updateH *updatecampaigndraft.Handler,
	scheduleH *schedulecampaign.Handler,
	cancelH *cancelcampaign.Handler,
	pauseH *pausecampaign.Handler,
	resumeH *resumecampaign.Handler,
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
	result, err := b.create.Execute(ctx, createcampaign.Command{
		WorkspaceID:    input.WorkspaceID,
		UserID:         input.UserID,
		Name:           input.Name,
		AudienceRef:    input.AudienceRef,
		TemplateID:     input.TemplateID,
		SenderDomainID: input.SenderDomainID,
		MessageType:    input.MessageType,
		Now:            input.Now,
	})
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *result}, nil
}

func (b *commandBus) UpdateCampaignDraft(ctx context.Context, input UpdateCampaignDraftInput) (*CampaignResult, error) {
	cmd := updatecampaigndraft.Command{
		WorkspaceID:    input.WorkspaceID,
		CampaignID:     input.CampaignID,
		UserID:         input.UserID,
		Name:           input.Name,
		AudienceRef:    input.AudienceRef,
		TemplateID:     input.TemplateID,
		SenderDomainID: input.SenderDomainID,
		MessageType:    input.MessageType,
		Now:            input.Now,
	}
	result, err := b.update.Execute(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return &CampaignResult{Campaign: *result}, nil
}

func (b *commandBus) ScheduleCampaign(ctx context.Context, input ScheduleCampaignInput) (*CampaignResult, error) {
	result, err := b.schedule.Execute(ctx, schedulecampaign.Command{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      input.UserID,
		ScheduledAt: input.ScheduledAt,
		Now:         input.Now,
	})
	if err != nil {
		return nil, err
	}
	return &CampaignResult{
		Campaign:       *result.Campaign,
		CandidateCount: int64(len(result.Candidates)),
	}, nil
}

func (b *commandBus) CancelCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	result, err := b.cancel.Execute(ctx, cancelcampaign.Command{
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
	result, err := b.pause.Execute(ctx, pausecampaign.Command{
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
	result, err := b.resume.Execute(ctx, resumecampaign.Command{
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
	list           *listcampaigns.Handler
	get            *getcampaign.Handler
	listCandidates *listcampaigncandidates.Handler
}

func newQueryBus(
	listH *listcampaigns.Handler,
	getH *getcampaign.Handler,
	listCandidatesH *listcampaigncandidates.Handler,
) QueryBus {
	return &queryBus{
		list:           listH,
		get:            getH,
		listCandidates: listCandidatesH,
	}
}

func (b *queryBus) ListCampaigns(ctx context.Context, input ListInput, userID string) (*CampaignListResult, error) {
	result, cursor, err := b.list.Execute(ctx, listcampaigns.Command{
		WorkspaceID:    input.WorkspaceID,
		UserID:         userID,
		Status:         input.Status,
		SenderDomainID: input.SenderDomainID,
		TemplateID:     input.TemplateID,
		From:           input.From,
		To:             input.To,
		Limit:          input.Limit,
		Cursor:         input.Cursor,
	})
	if err != nil {
		return nil, err
	}
	return &CampaignListResult{Campaigns: result, NextCursor: cursor}, nil
}

func (b *queryBus) GetCampaign(ctx context.Context, workspaceID, campaignID, userID string) (*CampaignResult, error) {
	campaign, count, err := b.get.Execute(ctx, getcampaign.Command{
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
	candidates, cursor, err := b.listCandidates.Execute(ctx, listcampaigncandidates.Command{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      userID,
		Status:      input.Status,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	})
	if err != nil {
		return nil, err
	}
	return &CandidateListResult{Candidates: candidates, NextCursor: cursor}, nil
}
