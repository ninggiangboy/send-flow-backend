package app

import (
	"context"
	"strings"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

func (s *Service) RefreshSenderDomainDNSStatus(ctx context.Context, workspaceID, domainID, actorUserID string, now time.Time) (*Result, error) {
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, actorUserID, "sender.manage"); err != nil {
		return nil, err
	}

	sd, records, err := s.domainsRead.FindByID(ctx, workspaceID, domainID)
	if err != nil {
		return nil, err
	}

	if sd.Status == senderdomain.SenderDomainStatusDisabled {
		return nil, senderdomain.ErrInvalidStateTransition
	}

	allVerified := true
	for i := range records {
		rec := &records[i]
		switch rec.RecordType {
		case senderdomain.DNSRecordTypeTXT:
			values, lookupErr := s.dnsResolver.LookupTXT(ctx, rec.Host)
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
			target, lookupErr := s.dnsResolver.LookupCNAME(ctx, rec.Host)
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
		rec.LastCheckedAt = &now
		rec.UpdatedAt = now
	}

	if allVerified {
		sd.Status = senderdomain.SenderDomainStatusVerified
		sd.VerifiedAt = &now
		s.log.Info("sender domain verified", "workspace_id", workspaceID, "sender_domain_id", domainID)
		if s.metricsRecorder != nil {
			s.metricsRecorder.RecordVerificationAttempt("verified")
		}
	} else if sd.Status == senderdomain.SenderDomainStatusVerified {
		sd.Status = senderdomain.SenderDomainStatusPendingVerification
		sd.VerifiedAt = nil
		s.log.Warn("sender domain reverted to pending", "workspace_id", workspaceID, "sender_domain_id", domainID)
		if s.metricsRecorder != nil {
			s.metricsRecorder.RecordVerificationAttempt("reverted")
		}
	} else {
		if s.metricsRecorder != nil {
			s.metricsRecorder.RecordVerificationAttempt("pending")
		}
	}
	sd.UpdatedAt = now

	if err := s.domainsWrite.UpdateDomain(ctx, *sd); err != nil {
		return nil, err
	}
	if err := s.domainsWrite.ReplaceDNSRecordStatuses(ctx, sd.ID, records); err != nil {
		return nil, err
	}

	return buildResult(*sd, records, now), nil
}
