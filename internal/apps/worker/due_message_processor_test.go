package worker

import (
	"context"
	"testing"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
)

func TestDueMessageProcessor_ProcessOnce(t *testing.T) {
	svc := deliveryapp.NewService(deliveryapp.Options{
		MessagesRead: &mockMessageReadRepo{},
		Logger:       testConsumerLogger(),
	})
	p := NewDueMessageProcessor(svc, testConsumerLogger(), time.Minute, 100, "marketing")

	p.processOnce(context.Background())
}
