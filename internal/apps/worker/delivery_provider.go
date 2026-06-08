package worker

import (
	"context"
	"fmt"
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
	err := a.sender.Send(ctx, email.Message{
		To:      []string{req.To},
		Subject: req.Subject,
		Text:    req.TextBody,
		HTML:    req.HTMLBody,
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
