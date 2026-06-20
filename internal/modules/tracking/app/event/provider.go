package event

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type ProviderOptions struct {
	RecordOpenH        *OpenHandler
	RecordClickH       *ClickHandler
	RecordUnsubscribeH *UnsubscribeHandler
	Logger             *slog.Logger
}

type ProviderHandler struct {
	recordOpenH        *OpenHandler
	recordClickH       *ClickHandler
	recordUnsubscribeH *UnsubscribeHandler
	log                *slog.Logger
}

func NewProvider(opts ProviderOptions) *ProviderHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &ProviderHandler{
		recordOpenH:        opts.RecordOpenH,
		recordClickH:       opts.RecordClickH,
		recordUnsubscribeH: opts.RecordUnsubscribeH,
		log:                opts.Logger.With("usecase", "handle_provider_event"),
	}
}

type ProviderInput struct {
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

type ProviderResult struct {
	Handled bool
	Ignored bool
}

func (h *ProviderHandler) Execute(ctx context.Context, input ProviderInput) (*ProviderResult, error) {
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
		_, err := h.recordOpenH.Execute(ctx, OpenInput{
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
		return &ProviderResult{Handled: true}, nil

	case "clicked":
		_, err := h.recordClickH.Execute(ctx, ClickInput{
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
		return &ProviderResult{Handled: true}, nil

	case "unsubscribed":
		_, err := h.recordUnsubscribeH.Execute(ctx, UnsubscribeInput{
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
		return &ProviderResult{Handled: true}, nil

	default:
		log.Info("event type not recognized for tracking, ignoring")
		return &ProviderResult{Ignored: true}, nil
	}
}
