package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/createtrackinglink"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/handleproviderevent"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/recordclick"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/recordopen"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/recordunsubscribe"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/unsubscribetoken"
	trackingdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
)

// Type aliases re-exported for external callers.
type (
	SuppressFromSignalInput  = recordunsubscribe.SuppressFromSignalInput
	SuppressFromSignalResult = recordunsubscribe.SuppressFromSignalResult
	RecipientSuppressor      = recordunsubscribe.RecipientSuppressor
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
	createTrackingLinkH  *createtrackinglink.Handler
	recordOpenH          *recordopen.Handler
	recordClickH         *recordclick.Handler
	recordUnsubscribeH   *recordunsubscribe.Handler
	handleProviderEventH *handleproviderevent.Handler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}

	recordOpenH := recordopen.New(recordopen.Options{
		LinkReadRepo:    opts.LinkReadRepo,
		EventReadRepo:   opts.EventReadRepo,
		EventWriteRepo:  opts.EventWriteRepo,
		MessageResolver: opts.MessageResolver,
		OutboxWriter:    opts.OutboxWriter,
		TxManager:       opts.TxManager,
		IDGen:           opts.IDGen,
		Logger:          opts.Logger,
	})
	recordClickH := recordclick.New(recordclick.Options{
		LinkReadRepo:    opts.LinkReadRepo,
		EventReadRepo:   opts.EventReadRepo,
		EventWriteRepo:  opts.EventWriteRepo,
		MessageResolver: opts.MessageResolver,
		OutboxWriter:    opts.OutboxWriter,
		TxManager:       opts.TxManager,
		IDGen:           opts.IDGen,
		Logger:          opts.Logger,
	})
	recordUnsubscribeH := recordunsubscribe.New(recordunsubscribe.Options{
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
		createTrackingLinkH: createtrackinglink.New(createtrackinglink.Options{
			LinkWriteRepo: opts.LinkWriteRepo,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		recordOpenH:        recordOpenH,
		recordClickH:       recordClickH,
		recordUnsubscribeH: recordUnsubscribeH,
		handleProviderEventH: handleproviderevent.New(handleproviderevent.Options{
			RecordOpenH:        recordOpenH,
			RecordClickH:       recordClickH,
			RecordUnsubscribeH: recordUnsubscribeH,
			Logger:             opts.Logger,
		}),
	}
}

// CreateTrackingLinkInput is the input to CreateTrackingLink.
// Deprecated: Prefer using createtrackinglink.Input directly.
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
	return s.createTrackingLinkH.Execute(ctx, createtrackinglink.Input{
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
// Deprecated: Prefer using recordopen.Input directly.
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
// Deprecated: Prefer using recordopen.Result directly.
type RecordOpenResult = recordopen.Result

func (s *Service) RecordOpen(ctx context.Context, input RecordOpenInput) (*RecordOpenResult, error) {
	return s.recordOpenH.Execute(ctx, recordopen.Input{
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
// Deprecated: Prefer using recordclick.Input directly.
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
// Deprecated: Prefer using recordclick.Result directly.
type RecordClickResult = recordclick.Result

func (s *Service) RecordClick(ctx context.Context, input RecordClickInput) (*RecordClickResult, error) {
	return s.recordClickH.Execute(ctx, recordclick.Input{
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
// Deprecated: Prefer using recordunsubscribe.Input directly.
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
// Deprecated: Prefer using recordunsubscribe.Result directly.
type RecordUnsubscribeResult = recordunsubscribe.Result

func (s *Service) RecordUnsubscribe(ctx context.Context, input RecordUnsubscribeInput) (*RecordUnsubscribeResult, error) {
	return s.recordUnsubscribeH.Execute(ctx, recordunsubscribe.Input{
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
// Deprecated: Prefer using handleproviderevent.Input directly.
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
// Deprecated: Prefer using handleproviderevent.Result directly.
type HandleProviderEventResult = handleproviderevent.Result

func (s *Service) HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error) {
	return s.handleProviderEventH.Execute(ctx, handleproviderevent.Input{
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
