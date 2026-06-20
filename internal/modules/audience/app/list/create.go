package list

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type CreateOptions struct {
	ListsWrite    ports.ListWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type CreateCommand struct {
	WorkspaceID string
	UserID      string
	Name        string
	Description string
	Metadata    map[string]any
	Now         time.Time
}

type CreateHandler struct {
	listsWrite    ports.ListWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	log           *slog.Logger
}

func NewCreate(opts CreateOptions) *CreateHandler {
	return &CreateHandler{
		listsWrite:    opts.ListsWrite,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("usecase", "create_list"),
	}
}

func (h *CreateHandler) Execute(ctx context.Context, cmd CreateCommand) (*domain.AudienceList, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if cmd.Name == "" {
		return nil, domain.ErrContactPayloadInvalid
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	list := domain.AudienceList{
		ID:          id,
		WorkspaceID: cmd.WorkspaceID,
		Name:        cmd.Name,
		Description: cmd.Description,
		Metadata:    cmd.Metadata,
		CreatedAt:   cmd.Now,
		UpdatedAt:   cmd.Now,
	}

	if err := h.listsWrite.CreateList(ctx, list); err != nil {
		if errors.Is(err, domain.ErrListNameConflict) {
			return nil, err
		}
		h.log.Error("failed to create list", "error", err)
		return nil, err
	}

	h.log.Info("list created", "list_id", id)
	return &list, nil
}
