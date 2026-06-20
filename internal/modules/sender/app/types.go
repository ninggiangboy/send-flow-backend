package app

import (
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/read"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/registration"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

// MetricsRecorder is used by sender domain registration handlers.
type MetricsRecorder = registration.MetricsRecorder

// Result combines a sender domain with its DNS records and readiness.
type Result struct {
	Domain    senderdomain.SenderDomain
	Records   []senderdomain.DNSRecord
	Readiness Readiness
}

// Readiness indicates whether a sender domain is ready to send.
type Readiness struct {
	Ready     bool
	Reason    string
	CheckedAt time.Time
}

// Input type aliases for backward compatibility.
type (
	CreateSenderDomainInput           = registration.CreateInput
	DisableSenderDomainInput          = registration.DisableInput
	RefreshSenderDomainDNSStatusInput = registration.RefreshInput
	GetSenderDomainInput              = read.GetInput
	ListSenderDomainsInput            = read.ListInput
	GetSenderReadinessInput           = read.ReadinessInput
)
