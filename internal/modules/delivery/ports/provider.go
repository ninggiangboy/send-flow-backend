package ports

import (
	"context"
	"io"
	"time"
)

type AttachmentPart struct {
	Filename    string
	ContentType string
	ContentID   string
	Disposition string
	Data        io.Reader
}

type ProviderSendRequest struct {
	To          []string
	CC          []string
	BCC         []string
	ReplyTo     []string
	Subject     string
	HTMLBody    string
	TextBody    string
	SenderName  string
	Headers     map[string]string
	Attachments []AttachmentPart
}

type ProviderSendResult struct {
	Provider          string
	ProviderMessageID string
	AcceptedAt        time.Time
}

type EmailProvider interface {
	SendEmail(ctx context.Context, request ProviderSendRequest) (*ProviderSendResult, error)
}
