package ports

import "context"

type SenderReadiness struct {
	Ready     bool
	Reason    string
	CheckedAt string
}

type SenderService interface {
	GetSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) (*SenderReadiness, error)
}
