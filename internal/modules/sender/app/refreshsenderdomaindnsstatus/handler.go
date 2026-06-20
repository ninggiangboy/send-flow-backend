package refreshsenderdomaindnsstatus

import (
	"context"
	"log/slog"
	"strings"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/ports"
)

type MetricsRecorder interface {
	RecordVerificationAttempt(result string)
	SetPendingDomains(count int64)
}

type Options struct {
	DomainsWrite    ports.SenderDomainWriteRepository
	DNSResolver     ports.DNSResolver
	AccessChecker   ports.WorkspaceAccessChecker
	MetricsRecorder MetricsRecorder
	Logger          *slog.Logger
}

type Command struct {
	WorkspaceID string
	DomainID    string
	ActorUserID string
	Now         time.Time
}

type Handler struct {
	domainsWrite    ports.SenderDomainWriteRepository
	dnsResolver     ports.DNSResolver
	accessChecker   ports.WorkspaceAccessChecker
	metricsRecorder MetricsRecorder
	log             *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		domainsWrite:    opts.DomainsWrite,
		dnsResolver:     opts.DNSResolver,
		accessChecker:   opts.AccessChecker,
		metricsRecorder: opts.MetricsRecorder,
		log:             opts.Logger.With("usecase", "refreshsenderdomaindnsstatus"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "sender.manage"); err != nil {
		return nil, nil, err
	}

	sd, records, err := h.domainsWrite.FindByID(ctx, cmd.WorkspaceID, cmd.DomainID)
	if err != nil {
		return nil, nil, err
	}

	if sd.Status == senderdomain.SenderDomainStatusDisabled {
		return nil, nil, senderdomain.ErrInvalidStateTransition
	}

	allVerified := true
	for i := range records {
		rec := &records[i]
		switch rec.RecordType {
		case senderdomain.DNSRecordTypeTXT:
			values, lookupErr := h.dnsResolver.LookupTXT(ctx, rec.Host)
			if lookupErr != nil {
				rec.CurrentValue = ""
				rec.Status = senderdomain.DNSRecordStatusMissing
				rec.FailureReason = lookupErr.Error()
				allVerified = false
			} else {
				var match bool
				for _, v := range values {
					trimmed := strings.TrimSpace(v)
					if strings.EqualFold(trimmed, strings.TrimSpace(rec.ExpectedValue)) {
						rec.CurrentValue = trimmed
						rec.Status = senderdomain.DNSRecordStatusVerified
						rec.FailureReason = ""
						match = true
						break
					}
				}
				if !match {
					rec.CurrentValue = strings.Join(values, "")
					rec.Status = senderdomain.DNSRecordStatusMismatch
					rec.FailureReason = "expected value does not match"
					allVerified = false
				}
			}
		case senderdomain.DNSRecordTypeCNAME:
			target, lookupErr := h.dnsResolver.LookupCNAME(ctx, rec.Host)
			if lookupErr != nil {
				rec.CurrentValue = ""
				rec.Status = senderdomain.DNSRecordStatusMissing
				rec.FailureReason = lookupErr.Error()
				allVerified = false
			} else {
				normalized := senderdomain.NormalizeCNAME(target)
				rec.CurrentValue = normalized
				if strings.EqualFold(normalized, strings.TrimSpace(rec.ExpectedValue)) {
					rec.Status = senderdomain.DNSRecordStatusVerified
					rec.FailureReason = ""
				} else {
					rec.Status = senderdomain.DNSRecordStatusMismatch
					rec.FailureReason = "expected value does not match"
					allVerified = false
				}
			}
		}
		rec.LastCheckedAt = &cmd.Now
		rec.UpdatedAt = cmd.Now
	}

	if allVerified {
		sd.Status = senderdomain.SenderDomainStatusVerified
		sd.VerifiedAt = &cmd.Now
		h.log.Info("sender domain verified", "workspace_id", cmd.WorkspaceID, "sender_domain_id", cmd.DomainID)
		if h.metricsRecorder != nil {
			h.metricsRecorder.RecordVerificationAttempt("verified")
		}
	} else if sd.Status == senderdomain.SenderDomainStatusVerified {
		sd.Status = senderdomain.SenderDomainStatusPendingVerification
		sd.VerifiedAt = nil
		h.log.Warn("sender domain reverted to pending", "workspace_id", cmd.WorkspaceID, "sender_domain_id", cmd.DomainID)
		if h.metricsRecorder != nil {
			h.metricsRecorder.RecordVerificationAttempt("reverted")
		}
	} else {
		if h.metricsRecorder != nil {
			h.metricsRecorder.RecordVerificationAttempt("pending")
		}
	}
	sd.UpdatedAt = cmd.Now

	if err := h.domainsWrite.UpdateDomain(ctx, *sd); err != nil {
		return nil, nil, err
	}
	if err := h.domainsWrite.ReplaceDNSRecordStatuses(ctx, sd.ID, records); err != nil {
		return nil, nil, err
	}

	return sd, records, nil
}
