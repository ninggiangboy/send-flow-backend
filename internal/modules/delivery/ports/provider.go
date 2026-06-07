package ports

import (
	"context"
	"time"
)

type ProviderSendRequest struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
	Headers  map[string]string
}

type ProviderSendResult struct {
	Provider          string
	ProviderMessageID string
	AcceptedAt        time.Time
}

type EmailProvider interface {
	SendEmail(ctx context.Context, request ProviderSendRequest) (*ProviderSendResult, error)
}
