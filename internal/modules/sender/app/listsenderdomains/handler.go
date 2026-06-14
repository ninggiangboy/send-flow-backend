package listsenderdomains

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
		log:           opts.Logger.With("usecase", "listsenderdomains"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]senderdomain.SenderDomain, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "sender.manage"); err != nil {
		return nil, err
	}

	domains, err := h.domainsRead.ListByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, err
	}

	return domains, nil
}
