package getsenderreadiness

import (
	"context"
	"log/slog"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/ports"
)

type Options struct {
	DomainsRead ports.SenderDomainReadRepository
	Logger      *slog.Logger
}

type Command struct {
	WorkspaceID   string
	SenderDomainID string
}

type Handler struct {
	domainsRead ports.SenderDomainReadRepository
	log         *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		domainsRead: opts.DomainsRead,
		log:         opts.Logger.With("usecase", "getsenderreadiness"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	sd, records, err := h.domainsRead.FindByID(ctx, cmd.WorkspaceID, cmd.SenderDomainID)
	if err != nil {
		return nil, nil, err
	}
	if sd == nil {
		return nil, nil, senderdomain.ErrDomainNotFound
	}
	return sd, records, nil
}
