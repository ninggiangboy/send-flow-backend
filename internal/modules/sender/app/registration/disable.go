package registration

import (
	"context"
	"log/slog"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/ports"
)

type DisableOptions struct {
	DomainsWrite  ports.SenderDomainWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type DisableInput struct {
	WorkspaceID string
	DomainID    string
	ActorUserID string
	Now         time.Time
}

type DisableHandler struct {
	domainsWrite  ports.SenderDomainWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewDisableHandler(opts DisableOptions) *DisableHandler {
	return &DisableHandler{
		domainsWrite:  opts.DomainsWrite,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "disablesenderdomain"),
	}
}

func (h *DisableHandler) Execute(ctx context.Context, cmd DisableInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "sender.manage"); err != nil {
		return nil, nil, err
	}

	sd, records, err := h.domainsWrite.FindByID(ctx, cmd.WorkspaceID, cmd.DomainID)
	if err != nil {
		return nil, nil, err
	}

	if !sd.CanTransitionTo(senderdomain.SenderDomainStatusDisabled) {
		return nil, nil, senderdomain.ErrInvalidStateTransition
	}

	sd.Status = senderdomain.SenderDomainStatusDisabled
	sd.DisabledAt = &cmd.Now
	sd.UpdatedAt = cmd.Now

	if err := h.domainsWrite.UpdateDomain(ctx, *sd); err != nil {
		return nil, nil, err
	}

	h.log.Info("sender domain disabled", "workspace_id", cmd.WorkspaceID, "sender_domain_id", cmd.DomainID)
	return sd, records, nil
}
