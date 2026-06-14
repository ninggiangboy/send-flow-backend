package app

import (
	"context"
	"errors"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

func (s *Service) CreateSenderDomain(ctx context.Context, workspaceID, actorUserID, rawDomain, provider string, now time.Time) (*Result, error) {
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, actorUserID, "sender.manage"); err != nil {
		return nil, err
	}

	normalizedDomain, err := senderdomain.NormalizeDomain(rawDomain)
	if err != nil {
		return nil, err
	}

	providerVal, err := senderdomain.ValidateProvider(provider)
	if err != nil {
		return nil, err
	}

	existing, err := s.domainsRead.FindByDomain(ctx, workspaceID, normalizedDomain)
	if err != nil && !errors.Is(err, senderdomain.ErrDomainNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, senderdomain.ErrDomainConflict
	}

	id, err := s.idGen()
	if err != nil {
		return nil, err
	}

	sd := senderdomain.SenderDomain{
		ID:          id,
		WorkspaceID: workspaceID,
		Domain:      normalizedDomain,
		Provider:    providerVal,
		Status:      senderdomain.SenderDomainStatusPendingVerification,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	records := senderdomain.GenerateDNSRecords(id, normalizedDomain, now)

	if err := s.domainsWrite.Create(ctx, sd, records); err != nil {
		return nil, err
	}

	s.log.Info("sender domain created", "workspace_id", workspaceID, "sender_domain_id", id, "domain", normalizedDomain)
	if s.metricsRecorder != nil {
		s.metricsRecorder.SetPendingDomains(0)
		s.metricsRecorder.RecordVerificationAttempt("created")
	}
	return buildResult(sd, records, now), nil
}
