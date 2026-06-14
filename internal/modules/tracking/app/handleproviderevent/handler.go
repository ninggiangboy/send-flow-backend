package handleproviderevent

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/recordclick"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/recordopen"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/recordunsubscribe"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
	"time"
)

type Options struct {
	RecordOpenH        *recordopen.Handler
	RecordClickH       *recordclick.Handler
	RecordUnsubscribeH *recordunsubscribe.Handler
	Logger             *slog.Logger
}

type Handler struct {
	recordOpenH        *recordopen.Handler
	recordClickH       *recordclick.Handler
	recordUnsubscribeH *recordunsubscribe.Handler
	log                *slog.Logger
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		recordOpenH:        opts.RecordOpenH,
		recordClickH:       opts.RecordClickH,
		recordUnsubscribeH: opts.RecordUnsubscribeH,
		log:                opts.Logger.With("usecase", "handle_provider_event"),
	}
}

type Input struct {
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

type Result struct {
	Handled bool
	Ignored bool
}

func (h *Handler) Execute(ctx context.Context, input Input) (*Result, error) {
	log := h.log.With(
		"event_id", input.EventID,
		"normalized_event_id", input.NormalizedEventID,
		"provider", input.Provider,
		"event_type", input.EventType,
	)

	if input.EventID == "" || input.EventType == "" {
		return nil, &platformerrors.NonRetryableError{Err: domain.ErrTrackingEventInvalid}
	}

	switch input.EventType {
	case "opened":
		_, err := h.recordOpenH.Execute(ctx, recordopen.Input{
			WorkspaceID:       input.WorkspaceID,
			MessageID:         input.MessageID,
			Provider:          input.Provider,
			ProviderMessageID: input.ProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			Source:            domain.SourceProviderEvent,
			SourceEventID:     input.NormalizedEventID,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
		})
		if err != nil {
			return nil, err
		}
		return &Result{Handled: true}, nil

	case "clicked":
		_, err := h.recordClickH.Execute(ctx, recordclick.Input{
			WorkspaceID:       input.WorkspaceID,
			MessageID:         input.MessageID,
			Provider:          input.Provider,
			ProviderMessageID: input.ProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			Source:            domain.SourceProviderEvent,
			SourceEventID:     input.NormalizedEventID,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
		})
		if err != nil {
			return nil, err
		}
		return &Result{Handled: true}, nil

	case "unsubscribed":
		_, err := h.recordUnsubscribeH.Execute(ctx, recordunsubscribe.Input{
			WorkspaceID:       input.WorkspaceID,
			MessageID:         input.MessageID,
			Provider:          input.Provider,
			ProviderMessageID: input.ProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			Source:            domain.SourceProviderEvent,
			SourceEventID:     input.NormalizedEventID,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
		})
		if err != nil {
			return nil, err
		}
		return &Result{Handled: true}, nil

	default:
		log.Info("event type not recognized for tracking, ignoring")
		return &Result{Ignored: true}, nil
	}
}
