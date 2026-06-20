package entry

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type CreateSystemInput struct {
	WorkspaceID     string
	Email           string
	EmailNormalized string
	Scope           string
	Reason          string
	Source          string
	SourceEventID   string
	Note            string
	Now             time.Time
}

type CreateSystemHandler struct {
	entriesWrite ports.SuppressionWriteRepository
	idGen        func() (string, error)
	log          *slog.Logger
}

func NewCreateSystemHandler(opts struct {
	EntriesWrite ports.SuppressionWriteRepository
	IDGen        func() (string, error)
	Logger       *slog.Logger
}) *CreateSystemHandler {
	return &CreateSystemHandler{
		entriesWrite: opts.EntriesWrite,
		idGen:        opts.IDGen,
		log:          opts.Logger.With("usecase", "create_system_suppression_entry"),
	}
}

func (h *CreateSystemHandler) Execute(ctx context.Context, cmd CreateSystemInput) (*domain.SuppressionEntry, bool, error) {
	if !domain.ValidSuppressionScope(cmd.Scope) {
		return nil, false, domain.ErrScopeInvalid
	}
	if !domain.ValidSuppressionReason(cmd.Reason) {
		return nil, false, domain.ErrReasonInvalid
	}

	emailNormalized := cmd.EmailNormalized
	if emailNormalized == "" && cmd.Email != "" {
		emailNormalized = domain.NormalizeEmail(cmd.Email)
	}
	if emailNormalized == "" {
		return nil, false, domain.ErrScopeInvalid
	}

	existing, err := h.entriesWrite.FindActiveByEmail(ctx, ports.SuppressionCheckQuery{
		WorkspaceID:     cmd.WorkspaceID,
		EmailNormalized: emailNormalized,
		Scopes:          []string{cmd.Scope},
		Reasons:         []string{cmd.Reason},
	})
	if err != nil {
		h.log.Error("failed to check existing suppression", "error", err)
		return nil, false, err
	}
	if existing != nil {
		h.log.Info("active suppression entry already exists, skipping", "entry_id", existing.ID)
		return existing, false, nil
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, false, err
	}

	entry := domain.SuppressionEntry{
		ID:              id,
		WorkspaceID:     cmd.WorkspaceID,
		Email:           cmd.Email,
		EmailNormalized: emailNormalized,
		Scope:           domain.SuppressionScope(cmd.Scope),
		Reason:          domain.SuppressionReason(cmd.Reason),
		Status:          domain.SuppressionStatusActive,
		Note:            cmd.Note,
		CreatedAt:       cmd.Now,
		UpdatedAt:       cmd.Now,
	}

	if err := h.entriesWrite.Create(ctx, entry); err != nil {
		h.log.Error("failed to create system suppression entry", "error", err)
		return nil, false, err
	}

	h.log.Info("system suppression entry created",
		"entry_id", id,
		"source", cmd.Source,
		"source_event_id", cmd.SourceEventID,
	)
	return &entry, true, nil
}
