package read

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

type GetInput struct {
	WorkspaceID string
	DomainID    string
	ActorUserID string
}

type ListInput struct {
	WorkspaceID string
	ActorUserID string
}

type ReadinessInput struct {
	WorkspaceID    string
	SenderDomainID string
}

type QueryService struct {
	domainsRead   ports.SenderDomainReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewQueryService(opts Options) *QueryService {
	return &QueryService{
		domainsRead:   opts.DomainsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "senderdomain_query"),
	}
}

func (s *QueryService) GetSenderDomain(ctx context.Context, input GetInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.ActorUserID, "sender.manage"); err != nil {
		return nil, nil, err
	}

	sd, records, err := s.domainsRead.FindByID(ctx, input.WorkspaceID, input.DomainID)
	if err != nil {
		return nil, nil, err
	}

	return sd, records, nil
}

func (s *QueryService) ListSenderDomains(ctx context.Context, input ListInput) ([]senderdomain.SenderDomain, error) {
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.ActorUserID, "sender.manage"); err != nil {
		return nil, err
	}

	domains, err := s.domainsRead.ListByWorkspace(ctx, input.WorkspaceID)
	if err != nil {
		return nil, err
	}

	return domains, nil
}

func (s *QueryService) GetSenderReadiness(ctx context.Context, input ReadinessInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	sd, records, err := s.domainsRead.FindByID(ctx, input.WorkspaceID, input.SenderDomainID)
	if err != nil {
		return nil, nil, err
	}
	if sd == nil {
		return nil, nil, senderdomain.ErrDomainNotFound
	}
	return sd, records, nil
}
