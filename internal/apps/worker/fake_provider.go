package worker

import (
	"context"
	"time"

	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type FakeEmailProvider struct {
	idGen func() (string, error)
}

func NewFakeEmailProvider() *FakeEmailProvider {
	return &FakeEmailProvider{
		idGen: id.NewUUIDGenerator().New,
	}
}

func (p *FakeEmailProvider) SendEmail(ctx context.Context, request deliveryports.ProviderSendRequest) (*deliveryports.ProviderSendResult, error) {
	msgID, err := p.idGen()
	if err != nil {
		return nil, err
	}
	return &deliveryports.ProviderSendResult{
		Provider:          "fake",
		ProviderMessageID: msgID,
		AcceptedAt:        time.Now().UTC(),
	}, nil
}

var _ deliveryports.EmailProvider = (*FakeEmailProvider)(nil)
