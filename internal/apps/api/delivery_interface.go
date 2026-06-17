package api

import (
	"context"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

// deliveryService defines the interface for delivery operations used by HTTP handlers.
// This allows tests to provide mock implementations without the full dependency chain.
type deliveryService interface {
	ListMessages(ctx context.Context, input deliveryapp.ListMessagesInput) (*deliveryapp.ListMessagesResult, error)
	GetMessage(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error)
	AcceptTransactionalSend(ctx context.Context, input deliveryapp.AcceptTransactionalSendInput) (*deliveryapp.AcceptTransactionalSendResult, error)
	GetTransactionalMessage(ctx context.Context, input deliveryapp.GetTransactionalMessageInput) (*deliveryapp.GetTransactionalMessageResult, error)
	ListMessageEvents(ctx context.Context, input deliveryapp.ListMessageEventsInput) (*deliveryapp.ListMessageEventsResult, error)
	ListRequestMessages(ctx context.Context, input deliveryapp.ListRequestMessagesInput) (*deliveryapp.ListRequestMessagesResult, error)
	ListAttempts(ctx context.Context, input deliveryapp.ListAttemptsInput) ([]domain.DeliveryAttempt, error)
}
