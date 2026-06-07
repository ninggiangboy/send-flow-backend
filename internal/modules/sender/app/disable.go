package app

import (
	"context"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

func (s *Service) DisableSenderDomain(ctx context.Context, workspaceID, domainID, actorUserID string, now time.Time) (*Result, error) {
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, actorUserID, "sender.manage"); err != nil {
		return nil, err
	}

	sd, records, err := s.domainsRead.FindByID(ctx, workspaceID, domainID)
	if err != nil {
		return nil, err
	}

	if !sd.CanTransitionTo(senderdomain.SenderDomainStatusDisabled) {
		return nil, senderdomain.ErrInvalidStateTransition
	}

	sd.Status = senderdomain.SenderDomainStatusDisabled
	sd.DisabledAt = &now
	sd.UpdatedAt = now

	if err := s.domainsWrite.UpdateDomain(ctx, *sd); err != nil {
		return nil, err
	}

	s.log.Info("sender domain disabled", "workspace_id", workspaceID, "sender_domain_id", domainID)
	return buildResult(*sd, records, now), nil
}

func (s *Service) GetSenderDomain(ctx context.Context, workspaceID, domainID, actorUserID string) (*Result, error) {
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, actorUserID, "sender.manage"); err != nil {
		return nil, err
	}

	sd, records, err := s.domainsRead.FindByID(ctx, workspaceID, domainID)
	if err != nil {
		return nil, err
	}

	return buildResult(*sd, records, time.Now().UTC()), nil
}

func (s *Service) ListSenderDomains(ctx context.Context, workspaceID, actorUserID string) ([]Result, error) {
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, actorUserID, "sender.manage"); err != nil {
		return nil, err
	}

	domains, err := s.domainsRead.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(domains))
	now := time.Now().UTC()
	for _, sd := range domains {
		_, records, err := s.domainsRead.FindByID(ctx, workspaceID, sd.ID)
		if err != nil {
			return nil, err
		}
		results = append(results, *buildResult(sd, records, now))
	}
	return results, nil
}

func (s *Service) GetSenderReadiness(ctx context.Context, workspaceID, domainID string) (*Readiness, error) {
	sd, records, err := s.domainsRead.FindByID(ctx, workspaceID, domainID)
	if err != nil {
		return nil, err
	}

	ready := readinessFor(*sd, records, time.Now().UTC())
	return &ready, nil
}
