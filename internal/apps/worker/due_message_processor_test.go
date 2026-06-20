package worker

import (
	"context"
	"testing"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
)

func TestDueMessageProcessor_Poll(t *testing.T) {
	svc := deliveryapp.NewService(deliveryapp.Options{
		MessagesWrite: &mockMessageWrite{},
		Logger:        testConsumerLogger(),
	})
	p := NewDueMessageProcessor(svc, testConsumerLogger(), time.Minute, 100, "marketing")

	_, _ = p.Poll(context.Background())
}

func TestDueMessageRunnerNamesAreMessageTypeSpecific(t *testing.T) {
	marketingScheduler := NewDueMessageScheduler(nil, nil, testConsumerLogger(), "marketing", 50)
	if marketingScheduler.Name() != "delivery.due_message_scheduler" {
		t.Fatalf("unexpected marketing scheduler name: %q", marketingScheduler.Name())
	}

	transactionalScheduler := NewDueMessageScheduler(nil, nil, testConsumerLogger(), "transactional", 50)
	if transactionalScheduler.Name() != "delivery.due_message_scheduler_tx" {
		t.Fatalf("unexpected transactional scheduler name: %q", transactionalScheduler.Name())
	}

	svc := deliveryapp.NewService(deliveryapp.Options{
		MessagesWrite: &mockMessageWrite{},
		Logger:        testConsumerLogger(),
	})

	marketingProcessor := NewDueMessageProcessor(svc, testConsumerLogger(), time.Minute, 100, "marketing")
	if marketingProcessor.Name() != "delivery.process_due_messages" {
		t.Fatalf("unexpected marketing processor name: %q", marketingProcessor.Name())
	}
	if marketingProcessor.RunnerKey() != "delivery.due_messages.db_processor" {
		t.Fatalf("unexpected marketing processor key: %q", marketingProcessor.RunnerKey())
	}

	transactionalProcessor := NewDueMessageProcessor(svc, testConsumerLogger(), time.Minute, 100, "transactional")
	if transactionalProcessor.Name() != "delivery.process_due_messages_tx" {
		t.Fatalf("unexpected transactional processor name: %q", transactionalProcessor.Name())
	}
	if transactionalProcessor.RunnerKey() != "delivery.due_messages_tx.db_processor" {
		t.Fatalf("unexpected transactional processor key: %q", transactionalProcessor.RunnerKey())
	}
}
