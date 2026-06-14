package getsenderdomain

import (
	"context"
	"log/slog"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/ports"
)

type Options struct {
	DomainsRead   ports.SenderDomainReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	DomainID    string
	ActorUserID string
}

type Handler struct {
	domainsRead   ports.SenderDomainReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		domainsRead:   opts.DomainsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "getsenderdomain"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "sender.manage"); err != nil {
		return nil, nil, err
	}

	sd, records, err := h.domainsRead.FindByID(ctx, cmd.WorkspaceID, cmd.DomainID)
	if err != nil {
		return nil, nil, err
	}

	return sd, records, nil
}
