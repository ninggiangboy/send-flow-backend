package ports

import "context"

type SenderReadiness struct {
	Ready      bool
	DomainID   string
	VerifiedAt string
}

type SenderReadinessChecker interface {
	GetSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) (*SenderReadiness, error)
}
