package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/read"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/registration"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/ports"
	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

type Options struct {
	DomainsRead     ports.SenderDomainReadRepository
	DomainsWrite    ports.SenderDomainWriteRepository
	DNSResolver     ports.DNSResolver
	AccessChecker   ports.WorkspaceAccessChecker
	IDGen           func() (string, error)
	Logger          *slog.Logger
	MetricsRecorder MetricsRecorder
	CacheAside      *platformredis.CacheAside
}

type Service struct {
	commands   CommandBus
	queries    QueryBus
	cacheAside *platformredis.CacheAside
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) {
			return "", nil
		}
	}
	logger := opts.Logger

	createH := registration.NewCreateHandler(registration.CreateOptions{
		DomainsWrite:    opts.DomainsWrite,
		AccessChecker:   opts.AccessChecker,
		IDGen:           opts.IDGen,
		MetricsRecorder: opts.MetricsRecorder,
		Logger:          logger,
	})
	refreshH := registration.NewRefreshHandler(registration.RefreshOptions{
		DomainsWrite:    opts.DomainsWrite,
		DNSResolver:     opts.DNSResolver,
		AccessChecker:   opts.AccessChecker,
		MetricsRecorder: opts.MetricsRecorder,
		Logger:          logger,
	})
	disableH := registration.NewDisableHandler(registration.DisableOptions{
		DomainsWrite:  opts.DomainsWrite,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})
	querySvc := read.NewQueryService(read.Options{
		DomainsRead:   opts.DomainsRead,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})

	return &Service{
		commands:   newCommandBus(createH, refreshH, disableH),
		queries:    newQueryBus(querySvc),
		cacheAside: opts.CacheAside,
	}
}

func readinessFor(sd senderdomain.SenderDomain, records []senderdomain.DNSRecord, now time.Time) Readiness {
	ready := sd.IsReady(records)
	reason := ""
	if !ready {
		switch {
		case sd.Status == senderdomain.SenderDomainStatusPendingVerification:
			reason = "domain pending verification"
		case sd.Status == senderdomain.SenderDomainStatusDisabled:
			reason = "domain is disabled"
		case sd.DisabledAt != nil:
			reason = "domain is disabled"
		case sd.Status != senderdomain.SenderDomainStatusVerified:
			reason = "domain not verified"
		default:
			for _, rec := range records {
				if rec.Status != senderdomain.DNSRecordStatusVerified {
					reason = "DNS records not fully verified"
					break
				}
			}
		}
	}
	return Readiness{Ready: ready, Reason: reason, CheckedAt: now}
}

func buildResult(sd senderdomain.SenderDomain, records []senderdomain.DNSRecord, now time.Time) *Result {
	return &Result{
		Domain:    sd,
		Records:   records,
		Readiness: readinessFor(sd, records, now),
	}
}

// Facade methods — signatures preserved for API/worker backward compatibility.

func (s *Service) CreateSenderDomain(ctx context.Context, workspaceID, actorUserID, rawDomain, provider string, now time.Time) (*Result, error) {
	sd, records, err := s.commands.CreateSenderDomain(ctx, registration.CreateInput{
		WorkspaceID: workspaceID,
		ActorUserID: actorUserID,
		RawDomain:   rawDomain,
		Provider:    provider,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return buildResult(*sd, records, now), nil
}

func (s *Service) RefreshSenderDomainDNSStatus(ctx context.Context, workspaceID, domainID, actorUserID string, now time.Time) (*Result, error) {
	sd, records, err := s.commands.RefreshSenderDomainDNSStatus(ctx, registration.RefreshInput{
		WorkspaceID: workspaceID,
		DomainID:    domainID,
		ActorUserID: actorUserID,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return buildResult(*sd, records, now), nil
}

func (s *Service) DisableSenderDomain(ctx context.Context, workspaceID, domainID, actorUserID string, now time.Time) (*Result, error) {
	sd, records, err := s.commands.DisableSenderDomain(ctx, registration.DisableInput{
		WorkspaceID: workspaceID,
		DomainID:    domainID,
		ActorUserID: actorUserID,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return buildResult(*sd, records, now), nil
}

func (s *Service) GetSenderDomain(ctx context.Context, workspaceID, domainID, actorUserID string) (*Result, error) {
	sd, records, err := s.queries.GetSenderDomain(ctx, read.GetInput{
		WorkspaceID: workspaceID,
		DomainID:    domainID,
		ActorUserID: actorUserID,
	})
	if err != nil {
		return nil, err
	}
	return buildResult(*sd, records, time.Now().UTC()), nil
}

func (s *Service) ListSenderDomains(ctx context.Context, workspaceID, actorUserID string) ([]Result, error) {
	domains, err := s.queries.ListSenderDomains(ctx, read.ListInput{
		WorkspaceID: workspaceID,
		ActorUserID: actorUserID,
	})
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(domains))
	now := time.Now().UTC()
	for _, sd := range domains {
		_, records, err := s.queries.GetSenderDomain(ctx, read.GetInput{
			WorkspaceID: workspaceID,
			DomainID:    sd.ID,
			ActorUserID: actorUserID,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, *buildResult(sd, records, now))
	}
	return results, nil
}

func (s *Service) GetSenderDomainReadiness(ctx context.Context, workspaceID, senderDomainID string) (*Readiness, error) {
	if s.cacheAside != nil {
		key := platformredis.KeySenderDomainAuth(workspaceID, senderDomainID)
		var cached Readiness
		err := s.cacheAside.GetOrLoadJSON(ctx, "sender", "readiness", key, &cached, 5*time.Minute, func() (any, error) {
			sd, records, loadErr := s.queries.GetSenderReadiness(ctx, read.ReadinessInput{
				WorkspaceID:    workspaceID,
				SenderDomainID: senderDomainID,
			})
			if loadErr != nil {
				return nil, loadErr
			}
			r := readinessFor(*sd, records, time.Now())
			return &r, nil
		})
		if err != nil {
			return nil, err
		}
		return &cached, nil
	}

	sd, records, err := s.queries.GetSenderReadiness(ctx, read.ReadinessInput{
		WorkspaceID:    workspaceID,
		SenderDomainID: senderDomainID,
	})
	if err != nil {
		return nil, err
	}

	readiness := readinessFor(*sd, records, time.Now())
	return &readiness, nil
}

func (s *Service) GetSenderReadiness(ctx context.Context, workspaceID, domainID string) (*Readiness, error) {
	if s.cacheAside != nil {
		key := platformredis.KeySenderDomainAuth(workspaceID, domainID)
		var cached Readiness
		err := s.cacheAside.GetOrLoadJSON(ctx, "sender", "readiness", key, &cached, 5*time.Minute, func() (any, error) {
			sd, records, loadErr := s.queries.GetSenderReadiness(ctx, read.ReadinessInput{
				WorkspaceID:    workspaceID,
				SenderDomainID: domainID,
			})
			if loadErr != nil {
				return nil, loadErr
			}
			r := readinessFor(*sd, records, time.Now().UTC())
			return &r, nil
		})
		if err != nil {
			return nil, err
		}
		return &cached, nil
	}

	sd, records, err := s.queries.GetSenderReadiness(ctx, read.ReadinessInput{
		WorkspaceID:    workspaceID,
		SenderDomainID: domainID,
	})
	if err != nil {
		return nil, err
	}

	ready := readinessFor(*sd, records, time.Now().UTC())
	return &ready, nil
}
