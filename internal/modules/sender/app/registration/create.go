package registration

import (
	"context"
	"errors"
	"log/slog"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/ports"
)

type CreateOptions struct {
	DomainsWrite    ports.SenderDomainWriteRepository
	AccessChecker   ports.WorkspaceAccessChecker
	IDGen           func() (string, error)
	MetricsRecorder MetricsRecorder
	Logger          *slog.Logger
}

type CreateInput struct {
	WorkspaceID string
	ActorUserID string
	RawDomain   string
	Provider    string
	Now         time.Time
}

type CreateHandler struct {
	domainsWrite    ports.SenderDomainWriteRepository
	accessChecker   ports.WorkspaceAccessChecker
	idGen           func() (string, error)
	metricsRecorder MetricsRecorder
	log             *slog.Logger
}

func NewCreateHandler(opts CreateOptions) *CreateHandler {
	return &CreateHandler{
		domainsWrite:    opts.DomainsWrite,
		accessChecker:   opts.AccessChecker,
		idGen:           opts.IDGen,
		metricsRecorder: opts.MetricsRecorder,
		log:             opts.Logger.With("usecase", "createsenderdomain"),
	}
}

func (h *CreateHandler) Execute(ctx context.Context, cmd CreateInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "sender.manage"); err != nil {
		return nil, nil, err
	}

	normalizedDomain, err := senderdomain.NormalizeDomain(cmd.RawDomain)
	if err != nil {
		return nil, nil, err
	}

	providerVal, err := senderdomain.ValidateProvider(cmd.Provider)
	if err != nil {
		return nil, nil, err
	}

	existing, err := h.domainsWrite.FindByDomain(ctx, cmd.WorkspaceID, normalizedDomain)
	if err != nil && !errors.Is(err, senderdomain.ErrDomainNotFound) {
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, senderdomain.ErrDomainConflict
	}

	id, err := h.idGen()
	if err != nil {
		return nil, nil, err
	}

	sd := senderdomain.SenderDomain{
		ID:          id,
		WorkspaceID: cmd.WorkspaceID,
		Domain:      normalizedDomain,
		Provider:    providerVal,
		Status:      senderdomain.SenderDomainStatusPendingVerification,
		CreatedAt:   cmd.Now,
		UpdatedAt:   cmd.Now,
	}

	records := senderdomain.GenerateDNSRecords(id, normalizedDomain, cmd.Now)

	if err := h.domainsWrite.Create(ctx, sd, records); err != nil {
		return nil, nil, err
	}

	h.log.Info("sender domain created", "workspace_id", cmd.WorkspaceID, "sender_domain_id", id, "domain", normalizedDomain)
	if h.metricsRecorder != nil {
		h.metricsRecorder.SetPendingDomains(0)
		h.metricsRecorder.RecordVerificationAttempt("created")
	}
	return &sd, records, nil
}
