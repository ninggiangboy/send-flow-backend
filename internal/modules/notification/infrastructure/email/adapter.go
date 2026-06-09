package email

import (
	"context"

	platformemail "github.com/ninggiangboy/send-flow/backend/internal/platform/email"
)

type EmailAdapter struct {
	sender platformemail.Sender
}

func NewEmailAdapter(sender platformemail.Sender) *EmailAdapter {
	return &EmailAdapter{sender: sender}
}

func (a *EmailAdapter) SendNotificationEmail(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	msg := platformemail.Message{
		To:      to,
		Subject: subject,
		Text:    textBody,
		HTML:    htmlBody,
	}
	return a.sender.Send(ctx, msg)
}
