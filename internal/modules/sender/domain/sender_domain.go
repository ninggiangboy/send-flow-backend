package domain

import "time"

type SenderDomainStatus string

const (
	SenderDomainStatusPendingVerification SenderDomainStatus = "pending_verification"
	SenderDomainStatusVerified            SenderDomainStatus = "verified"
	SenderDomainStatusDisabled            SenderDomainStatus = "disabled"
)

type DNSRecordStatus string

const (
	DNSRecordStatusPending  DNSRecordStatus = "pending"
	DNSRecordStatusVerified DNSRecordStatus = "verified"
	DNSRecordStatusMissing  DNSRecordStatus = "missing"
	DNSRecordStatusMismatch DNSRecordStatus = "mismatch"
)

type DNSRecordType string

const (
	DNSRecordTypeTXT   DNSRecordType = "TXT"
	DNSRecordTypeCNAME DNSRecordType = "CNAME"
)

type Provider string

const (
	ProviderSES Provider = "ses"
)

type SenderDomain struct {
	ID          string
	WorkspaceID string
	Domain      string
	Provider    Provider
	Status      SenderDomainStatus
	VerifiedAt  *time.Time
	DisabledAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DNSRecord struct {
	ID             string
	SenderDomainID string
	RecordType     DNSRecordType
	Host           string
	ExpectedValue  string
	CurrentValue   string
	Status         DNSRecordStatus
	LastCheckedAt  *time.Time
	FailureReason  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (d SenderDomain) CanTransitionTo(target SenderDomainStatus) bool {
	switch d.Status {
	case SenderDomainStatusPendingVerification:
		return target == SenderDomainStatusVerified || target == SenderDomainStatusDisabled
	case SenderDomainStatusVerified:
		return target == SenderDomainStatusDisabled || target == SenderDomainStatusPendingVerification
	case SenderDomainStatusDisabled:
		return false
	default:
		return false
	}
}

func (d SenderDomain) IsReady(records []DNSRecord) bool {
	if d.Status != SenderDomainStatusVerified {
		return false
	}
	if d.DisabledAt != nil {
		return false
	}
	for _, rec := range records {
		if rec.Status != DNSRecordStatusVerified {
			return false
		}
	}
	return true
}
