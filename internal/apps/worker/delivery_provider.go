package worker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type deliveryEmailProviderAdapter struct {
	providerName string
	sender       email.Sender
	idGen        func() (string, error)
}

func newDeliveryEmailProvider(providerName string, sender email.Sender) *deliveryEmailProviderAdapter {
	return &deliveryEmailProviderAdapter{
		providerName: providerName,
		sender:       sender,
		idGen:        id.NewUUIDGenerator().New,
	}
}

func (a *deliveryEmailProviderAdapter) SendEmail(ctx context.Context, req deliveryports.ProviderSendRequest) (*deliveryports.ProviderSendResult, error) {
	// Add Reply-To header from the structured field if present.
	headers := req.Headers
	if len(req.ReplyTo) > 0 {
		if headers == nil {
			headers = make(map[string]string)
		}
		if _, exists := headers["Reply-To"]; !exists {
			headers["Reply-To"] = strings.Join(req.ReplyTo, ", ")
		}
	}

	// Read attachment data.
	attachments := make([]email.Attachment, 0, len(req.Attachments))
	for _, att := range req.Attachments {
		data, err := io.ReadAll(att.Data)
		if err != nil {
			return nil, fmt.Errorf("read attachment %q: %w", att.Filename, err)
		}
		disp := att.Disposition
		if disp == "" {
			disp = "attachment"
		}
		attachments = append(attachments, email.Attachment{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			ContentID:   att.ContentID,
			Disposition: disp,
			Data:        data,
		})
	}

	err := a.sender.Send(ctx, email.Message{
		To:          req.To,
		CC:          req.CC,
		BCC:         req.BCC,
		Subject:     req.Subject,
		Text:        req.TextBody,
		HTML:        req.HTMLBody,
		SenderName:  req.SenderName,
		Headers:     headers,
		Attachments: attachments,
	})
	if err != nil {
		return nil, fmt.Errorf("delivery provider send: %w", err)
	}

	msgID, err := a.idGen()
	if err != nil {
		return nil, err
	}

	return &deliveryports.ProviderSendResult{
		Provider:          a.providerName,
		ProviderMessageID: msgID,
		AcceptedAt:        time.Now().UTC(),
	}, nil
}

var _ deliveryports.EmailProvider = (*deliveryEmailProviderAdapter)(nil)
