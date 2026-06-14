package app

import (
	"context"
	"log/slog"
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
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
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

	createH := createcampaign.New(createcampaign.Options{
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		IDGen:          opts.IDGen,
		Logger:         log,
	})

	updateH := updatecampaigndraft.New(updatecampaigndraft.Options{
		CampaignsRead:  opts.CampaignsRead,
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	scheduleH := schedulecampaign.New(schedulecampaign.Options{
		CampaignsRead:    opts.CampaignsRead,
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

	cancelH := cancelcampaign.New(cancelcampaign.Options{
		CampaignsRead:  opts.CampaignsRead,
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	pauseH := pausecampaign.New(pausecampaign.Options{
		CampaignsRead:  opts.CampaignsRead,
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	resumeH := resumecampaign.New(resumecampaign.Options{
		CampaignsRead:  opts.CampaignsRead,
		CampaignsWrite: opts.CampaignsWrite,
		AccessChecker:  opts.AccessChecker,
		Logger:         log,
	})

	listH := listcampaigns.New(listcampaigns.Options{
		CampaignsRead: opts.CampaignsRead,
		AccessChecker: opts.AccessChecker,
		Logger:        log,
	})

	getH := getcampaign.New(getcampaign.Options{
		CampaignsRead: opts.CampaignsRead,
		AccessChecker: opts.AccessChecker,
		Logger:        log,
	})

	listCandidatesH := listcampaigncandidates.New(listcampaigncandidates.Options{
		CampaignsRead: opts.CampaignsRead,
		AccessChecker: opts.AccessChecker,
		Logger:        log,
	})

	return &Service{
		commands: newCommandBus(createH, updateH, scheduleH, cancelH, pauseH, resumeH),
		queries:  newQueryBus(listH, getH, listCandidatesH),
		log:      log,
	}
}

// Public input types

type CreateCampaignInput struct {
	WorkspaceID    string
	UserID         string
	Name           string
	AudienceRef    domain.AudienceRef
	TemplateID     string
	SenderDomainID string
	MessageType    domain.MessageType
	Now            time.Time
}

type UpdateCampaignDraftInput struct {
	WorkspaceID    string
	CampaignID     string
	UserID         string
	Name           *string
	AudienceRef    *domain.AudienceRef
	TemplateID     *string
	SenderDomainID *string
	MessageType    *domain.MessageType
	Now            time.Time
}

type ScheduleCampaignInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	ScheduledAt *time.Time
	Now         time.Time
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

// Public result types

type CampaignResult struct {
	Campaign       domain.Campaign
	CandidateCount int64
}

type CampaignListResult struct {
	Campaigns  []domain.Campaign
	NextCursor string
}

type CandidateListResult struct {
	Candidates []domain.CampaignMessageCandidate
	NextCursor string
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
