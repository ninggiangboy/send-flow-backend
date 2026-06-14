package createtrackinglink

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type Options struct {
	LinkWriteRepo ports.TrackingLinkWriteRepository
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type Handler struct {
	linkWriteRepo ports.TrackingLinkWriteRepository
	idGen         func() (string, error)
	log           *slog.Logger
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		linkWriteRepo: opts.LinkWriteRepo,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("usecase", "create_tracking_link"),
	}
}

type Input struct {
	WorkspaceID    string
	MessageID      string
	DestinationURL string
	LinkType       string
	Metadata       map[string]any
	Now            time.Time
	ExpiresAt      *time.Time
}

func (h *Handler) Execute(ctx context.Context, input Input) (*domain.TrackingLink, error) {
	log := h.log.With("workspace_id", input.WorkspaceID)

	if !domain.ValidLinkType(input.LinkType) {
		return nil, &platformerrors.NonRetryableError{Err: domain.ErrTrackingEventInvalid}
	}
	if input.DestinationURL != "" {
		if err := domain.ValidateDestinationURL(input.DestinationURL); err != nil {
			return nil, &platformerrors.NonRetryableError{Err: err}
		}
	}

	id, err := h.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	link := domain.TrackingLink{
		ID:             id,
		WorkspaceID:    input.WorkspaceID,
		MessageID:      input.MessageID,
		DestinationURL: input.DestinationURL,
		LinkType:       input.LinkType,
		Metadata:       input.Metadata,
		CreatedAt:      input.Now,
		ExpiresAt:      input.ExpiresAt,
	}

	if link.Metadata == nil {
		link.Metadata = map[string]any{}
	}

	if err := h.linkWriteRepo.Create(ctx, link); err != nil {
		log.Error("failed to create tracking link", "error", err)
		return nil, err
	}

	log.Info("tracking link created", "tracking_link_id", id, "link_type", input.LinkType)
	return &link, nil
}
