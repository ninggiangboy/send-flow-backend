package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

type SenderDomainReadRepository interface {
	FindByID(ctx context.Context, workspaceID, domainID string) (*domain.SenderDomain, []domain.DNSRecord, error)
	FindByDomain(ctx context.Context, workspaceID, normalizedDomain string) (*domain.SenderDomain, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.SenderDomain, error)
}

type SenderDomainWriteRepository interface {
	Create(ctx context.Context, senderDomain domain.SenderDomain, records []domain.DNSRecord) error
	UpdateDomain(ctx context.Context, senderDomain domain.SenderDomain) error
	ReplaceDNSRecordStatuses(ctx context.Context, senderDomainID string, records []domain.DNSRecord) error
}
