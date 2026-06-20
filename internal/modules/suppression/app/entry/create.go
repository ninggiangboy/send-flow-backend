package entry

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type CreateInput struct {
	WorkspaceID string
	UserID      string
	Email       string
	Scope       string
	Reason      string
	Note        string
	Now         time.Time
}

type CreateHandler struct {
	entriesWrite  ports.SuppressionWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	log           *slog.Logger
}

func NewCreateHandler(opts struct {
	EntriesWrite  ports.SuppressionWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Logger        *slog.Logger
}) *CreateHandler {
	return &CreateHandler{
		entriesWrite:  opts.EntriesWrite,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("usecase", "create_suppression_entry"),
	}
}

func (h *CreateHandler) Execute(ctx context.Context, cmd CreateInput) (*domain.SuppressionEntry, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "suppression.manage"); err != nil {
		return nil, err
	}

	if !domain.ValidSuppressionScope(cmd.Scope) {
		return nil, domain.ErrScopeInvalid
	}
	if !domain.ValidSuppressionReason(cmd.Reason) {
		return nil, domain.ErrReasonInvalid
	}

	if cmd.Email == "" {
		return nil, domain.ErrEmailInvalid
	}

	normalized := domain.NormalizeEmail(cmd.Email)

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	entry := domain.SuppressionEntry{
		ID:              id,
		WorkspaceID:     cmd.WorkspaceID,
		Email:           cmd.Email,
		EmailNormalized: normalized,
		Scope:           domain.SuppressionScope(cmd.Scope),
		Reason:          domain.SuppressionReason(cmd.Reason),
		Status:          domain.SuppressionStatusActive,
		Note:            cmd.Note,
		CreatedAt:       cmd.Now,
		UpdatedAt:       cmd.Now,
	}

	if err := h.entriesWrite.Create(ctx, entry); err != nil {
		h.log.Error("failed to create suppression entry", "error", err)
		return nil, err
	}

	h.log.Info("suppression entry created", "entry_id", id)
	return &entry, nil
}
