package app

import (
	"context"
	"log/slog"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/ports"
)

type MetricsRecorder interface {
	RecordVerificationAttempt(result string)
	SetPendingDomains(count int64)
}

type Options struct {
	DomainsRead     ports.SenderDomainReadRepository
	DomainsWrite    ports.SenderDomainWriteRepository
	DNSResolver     ports.DNSResolver
	AccessChecker   ports.WorkspaceAccessChecker
	IDGen           func() (string, error)
	Logger          *slog.Logger
	MetricsRecorder MetricsRecorder
}

type Result struct {
	Domain    senderdomain.SenderDomain
	Records   []senderdomain.DNSRecord
	Readiness Readiness
}

type Readiness struct {
	Ready     bool
	Reason    string
	CheckedAt time.Time
}

type Service struct {
	domainsRead     ports.SenderDomainReadRepository
	domainsWrite    ports.SenderDomainWriteRepository
	dnsResolver     ports.DNSResolver
	accessChecker   ports.WorkspaceAccessChecker
	idGen           func() (string, error)
	metricsRecorder MetricsRecorder
	log             *slog.Logger
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
	return &Service{
		domainsRead:     opts.DomainsRead,
		domainsWrite:    opts.DomainsWrite,
		dnsResolver:     opts.DNSResolver,
		accessChecker:   opts.AccessChecker,
		idGen:           opts.IDGen,
		metricsRecorder: opts.MetricsRecorder,
		log:             opts.Logger.With("module", "sender"),
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

func (s *Service) GetSenderDomainReadiness(ctx context.Context, workspaceID, senderDomainID string) (*Readiness, error) {
	sd, records, err := s.domainsRead.FindByID(ctx, workspaceID, senderDomainID)
	if err != nil {
		return nil, err
	}
	if sd == nil {
		return nil, senderdomain.ErrDomainNotFound
	}

	readiness := readinessFor(*sd, records, time.Now())
	return &readiness, nil
}

func buildResult(sd senderdomain.SenderDomain, records []senderdomain.DNSRecord, now time.Time) *Result {
	return &Result{
		Domain:    sd,
		Records:   records,
		Readiness: readinessFor(sd, records, now),
	}
}
