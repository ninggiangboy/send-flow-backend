package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/event"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/link"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/unsubscribetoken"
	trackingdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
)

// Type aliases re-exported for external callers.
type (
	SuppressFromSignalInput  = event.SuppressFromSignalInput
	SuppressFromSignalResult = event.SuppressFromSignalResult
	RecipientSuppressor      = event.RecipientSuppressor
)

type Options struct {
	LinkReadRepo        ports.TrackingLinkReadRepository
	LinkWriteRepo       ports.TrackingLinkWriteRepository
	EventReadRepo       ports.TrackingEventReadRepository
	EventWriteRepo      ports.TrackingEventWriteRepository
	MessageResolver     ports.DeliveryMessageResolver
	RecipientSuppressor RecipientSuppressor
	OutboxWriter        ports.OutboxWriter
	TxManager           ports.TransactionManager
	IDGen               func() (string, error)
	Logger              *slog.Logger
	TokenSigner         *unsubscribetoken.Signer
}

type Service struct {
	createTrackingLinkH  *link.CreateHandler
	recordOpenH          *event.OpenHandler
	recordClickH         *event.ClickHandler
	recordUnsubscribeH   *event.UnsubscribeHandler
	handleProviderEventH *event.ProviderHandler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}

	recordOpenH := event.NewOpen(event.OpenOptions{
		LinkReadRepo:    opts.LinkReadRepo,
		EventReadRepo:   opts.EventReadRepo,
		EventWriteRepo:  opts.EventWriteRepo,
		MessageResolver: opts.MessageResolver,
		OutboxWriter:    opts.OutboxWriter,
		TxManager:       opts.TxManager,
		IDGen:           opts.IDGen,
		Logger:          opts.Logger,
	})
	recordClickH := event.NewClick(event.ClickOptions{
		LinkReadRepo:    opts.LinkReadRepo,
		EventReadRepo:   opts.EventReadRepo,
		EventWriteRepo:  opts.EventWriteRepo,
		MessageResolver: opts.MessageResolver,
		OutboxWriter:    opts.OutboxWriter,
		TxManager:       opts.TxManager,
		IDGen:           opts.IDGen,
		Logger:          opts.Logger,
	})
	recordUnsubscribeH := event.NewUnsubscribe(event.UnsubscribeOptions{
		EventReadRepo:       opts.EventReadRepo,
		EventWriteRepo:      opts.EventWriteRepo,
		MessageResolver:     opts.MessageResolver,
		RecipientSuppressor: opts.RecipientSuppressor,
		OutboxWriter:        opts.OutboxWriter,
		TxManager:           opts.TxManager,
		IDGen:               opts.IDGen,
		TokenSigner:         opts.TokenSigner,
		Logger:              opts.Logger,
	})

	return &Service{
		createTrackingLinkH: link.NewCreate(link.CreateOptions{
			LinkWriteRepo: opts.LinkWriteRepo,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		recordOpenH:        recordOpenH,
		recordClickH:       recordClickH,
		recordUnsubscribeH: recordUnsubscribeH,
		handleProviderEventH: event.NewProvider(event.ProviderOptions{
			RecordOpenH:        recordOpenH,
			RecordClickH:       recordClickH,
			RecordUnsubscribeH: recordUnsubscribeH,
			Logger:             opts.Logger,
		}),
	}
}

// CreateTrackingLinkInput is the input to CreateTrackingLink.
// Deprecated: Prefer using link.CreateInput directly.
type CreateTrackingLinkInput struct {
	WorkspaceID    string
	MessageID      string
	DestinationURL string
	LinkType       string
	Metadata       map[string]any
	Now            time.Time
	ExpiresAt      *time.Time
}

func (s *Service) CreateTrackingLink(ctx context.Context, input CreateTrackingLinkInput) (*trackingdomain.TrackingLink, error) {
	return s.createTrackingLinkH.Execute(ctx, link.CreateInput{
		WorkspaceID:    input.WorkspaceID,
		MessageID:      input.MessageID,
		DestinationURL: input.DestinationURL,
		LinkType:       input.LinkType,
		Metadata:       input.Metadata,
		Now:            input.Now,
		ExpiresAt:      input.ExpiresAt,
	})
}

// RecordOpenInput is the input to RecordOpen.
// Deprecated: Prefer using event.OpenInput directly.
type RecordOpenInput struct {
	TrackingID        string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	NormalizedEventID string
	Source            string
	SourceEventID     string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

// RecordOpenResult is the result of RecordOpen.
// Deprecated: Prefer using event.OpenResult directly.
type RecordOpenResult = event.OpenResult

func (s *Service) RecordOpen(ctx context.Context, input RecordOpenInput) (*RecordOpenResult, error) {
	return s.recordOpenH.Execute(ctx, event.OpenInput{
		TrackingID:        input.TrackingID,
		WorkspaceID:       input.WorkspaceID,
		MessageID:         input.MessageID,
		Provider:          input.Provider,
		ProviderMessageID: input.ProviderMessageID,
		ProviderEventID:   input.ProviderEventID,
		NormalizedEventID: input.NormalizedEventID,
		Source:            input.Source,
		SourceEventID:     input.SourceEventID,
		OccurredAt:        input.OccurredAt,
		ReceivedAt:        input.ReceivedAt,
	})
}

// RecordClickInput is the input to RecordClick.
// Deprecated: Prefer using event.ClickInput directly.
type RecordClickInput struct {
	TrackingID        string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	NormalizedEventID string
	Source            string
	SourceEventID     string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

// RecordClickResult is the result of RecordClick.
// Deprecated: Prefer using event.ClickResult directly.
type RecordClickResult = event.ClickResult

func (s *Service) RecordClick(ctx context.Context, input RecordClickInput) (*RecordClickResult, error) {
	return s.recordClickH.Execute(ctx, event.ClickInput{
		TrackingID:        input.TrackingID,
		WorkspaceID:       input.WorkspaceID,
		MessageID:         input.MessageID,
		Provider:          input.Provider,
		ProviderMessageID: input.ProviderMessageID,
		ProviderEventID:   input.ProviderEventID,
		NormalizedEventID: input.NormalizedEventID,
		Source:            input.Source,
		SourceEventID:     input.SourceEventID,
		OccurredAt:        input.OccurredAt,
		ReceivedAt:        input.ReceivedAt,
	})
}

// RecordUnsubscribeInput is the input to RecordUnsubscribe.
// Deprecated: Prefer using event.UnsubscribeInput directly.
type RecordUnsubscribeInput struct {
	Token             string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	NormalizedEventID string
	Source            string
	SourceEventID     string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

// RecordUnsubscribeResult is the result of RecordUnsubscribe.
// Deprecated: Prefer using event.UnsubscribeResult directly.
type RecordUnsubscribeResult = event.UnsubscribeResult

func (s *Service) RecordUnsubscribe(ctx context.Context, input RecordUnsubscribeInput) (*RecordUnsubscribeResult, error) {
	return s.recordUnsubscribeH.Execute(ctx, event.UnsubscribeInput{
		Token:             input.Token,
		WorkspaceID:       input.WorkspaceID,
		MessageID:         input.MessageID,
		Provider:          input.Provider,
		ProviderMessageID: input.ProviderMessageID,
		ProviderEventID:   input.ProviderEventID,
		NormalizedEventID: input.NormalizedEventID,
		Source:            input.Source,
		SourceEventID:     input.SourceEventID,
		OccurredAt:        input.OccurredAt,
		ReceivedAt:        input.ReceivedAt,
	})
}

// HandleProviderEventInput is the input to HandleProviderEvent.
// Deprecated: Prefer using event.ProviderInput directly.
type HandleProviderEventInput struct {
	EventID           string
	NormalizedEventID string
	RawEventID        string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderEventID   string
	ProviderMessageID string
	EventType         string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

// HandleProviderEventResult is the result of HandleProviderEvent.
// Deprecated: Prefer using event.ProviderResult directly.
type HandleProviderEventResult = event.ProviderResult

func (s *Service) HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error) {
	return s.handleProviderEventH.Execute(ctx, event.ProviderInput{
		EventID:           input.EventID,
		NormalizedEventID: input.NormalizedEventID,
		RawEventID:        input.RawEventID,
		WorkspaceID:       input.WorkspaceID,
		MessageID:         input.MessageID,
		Provider:          input.Provider,
		ProviderEventID:   input.ProviderEventID,
		ProviderMessageID: input.ProviderMessageID,
		EventType:         input.EventType,
		OccurredAt:        input.OccurredAt,
		ReceivedAt:        input.ReceivedAt,
	})
}
