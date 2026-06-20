package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/draft"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app/read"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type Options struct {
	CampaignsRead    ports.CampaignReadRepository
	CampaignsWrite   ports.CampaignWriteRepository
	AudienceResolver ports.AudienceResolver
	ContentService   ports.ContentService
	SenderService    ports.SenderService
	OutboxWriter     ports.OutboxWriter
	TxManager        ports.UnitOfWork
	AccessChecker    ports.WorkspaceAccessChecker
	IDGen            func() (string, error)
	Logger           *slog.Logger
}

type Service struct {
	commands CommandBus
	queries  QueryBus
	log      *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}

	log := opts.Logger.With("module", "campaign")

	createH := draft.NewCreateHandler(draft.CreateOptions{
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		IDGen:          opts.IDGen,
		Logger:         log,
	})

	updateH := draft.NewUpdateHandler(draft.UpdateOptions{
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	scheduleH := draft.NewScheduleHandler(draft.ScheduleOptions{
		CampaignsWrite:   opts.CampaignsWrite,
		AudienceResolver: opts.AudienceResolver,
		ContentService:   opts.ContentService,
		SenderService:    opts.SenderService,
		OutboxWriter:     opts.OutboxWriter,
		TxManager:        opts.TxManager,
		AccessChecker:    opts.AccessChecker,
		IDGen:            opts.IDGen,
		Logger:           log,
	})

	cancelH := draft.NewCancelHandler(draft.CancelOptions{
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	pauseH := draft.NewPauseHandler(draft.PauseOptions{
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	resumeH := draft.NewResumeHandler(draft.ResumeOptions{
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	querySvc := read.NewQueryService(read.Options{
		CampaignsRead: opts.CampaignsRead,
		AccessChecker: opts.AccessChecker,
		Logger:        log,
	})

	return &Service{
		commands: newCommandBus(createH, updateH, scheduleH, cancelH, pauseH, resumeH),
		queries:  newQueryBus(querySvc),
		log:      log,
	}
}

// Facade methods delegate to buses

func (s *Service) CreateCampaign(ctx context.Context, input CreateCampaignInput) (*CampaignResult, error) {
	return s.commands.CreateCampaign(ctx, input)
}

func (s *Service) UpdateCampaignDraft(ctx context.Context, input UpdateCampaignDraftInput) (*CampaignResult, error) {
	return s.commands.UpdateCampaignDraft(ctx, input)
}

func (s *Service) ScheduleCampaign(ctx context.Context, input ScheduleCampaignInput) (*CampaignResult, error) {
	return s.commands.ScheduleCampaign(ctx, input)
}

func (s *Service) CancelCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	return s.commands.CancelCampaign(ctx, workspaceID, campaignID, userID, now)
}

func (s *Service) PauseCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	return s.commands.PauseCampaign(ctx, workspaceID, campaignID, userID, now)
}

func (s *Service) ResumeCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	return s.commands.ResumeCampaign(ctx, workspaceID, campaignID, userID, now)
}

func (s *Service) ListCampaigns(ctx context.Context, input ListInput, userID string) (*CampaignListResult, error) {
	return s.queries.ListCampaigns(ctx, input, userID)
}

func (s *Service) GetCampaign(ctx context.Context, workspaceID, campaignID, userID string) (*CampaignResult, error) {
	return s.queries.GetCampaign(ctx, workspaceID, campaignID, userID)
}

func (s *Service) ListCampaignCandidates(ctx context.Context, input CandidateListInput, userID string) (*CandidateListResult, error) {
	return s.queries.ListCampaignCandidates(ctx, input, userID)
}
