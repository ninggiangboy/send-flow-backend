package app

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/campaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/message"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/process"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/send"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

type CommandBus interface {
	AcceptTransactionalSend(ctx context.Context, input AcceptTransactionalSendInput) (*AcceptTransactionalSendResult, error)
	QueueCampaignMessages(ctx context.Context, input QueueCampaignMessagesInput) (*QueueCampaignMessagesResult, error)
	HandleCampaignScheduled(ctx context.Context, input HandleCampaignScheduledInput) error
	HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error)
	ProcessDueMessagesAllWorkspaces(ctx context.Context, input ProcessDueMessagesAllInput) (int, error)
	ProcessDueMessages(ctx context.Context, input ProcessDueMessagesInput) (*ProcessDueMessagesResult, error)
}

type QueryBus interface {
	ListMessages(ctx context.Context, input ListMessagesInput) (*ListMessagesResult, error)
	GetMessage(ctx context.Context, input GetMessageInput) (*domain.Message, error)
	GetTransactionalMessage(ctx context.Context, workspaceID, messageID string) (*GetTransactionalMessageResult, error)
}

type commandBus struct {
	logger              *slog.Logger
	acceptTransactional *send.Handler
	queueCampaign       *campaign.QueueCampaignMessagesHandler
	handleCampaign      *campaign.CampaignScheduledHandler
	handleProvider      *process.ProviderEventHandler
	processDueMsgs      *process.ProcessDueMessagesHandler
}

func newCommandBus(
	logger *slog.Logger,
	acceptTransactionalH *send.Handler,
	queueCampaignH *campaign.QueueCampaignMessagesHandler,
	handleCampaignH *campaign.CampaignScheduledHandler,
	handleProviderH *process.ProviderEventHandler,
	processDueMsgsH *process.ProcessDueMessagesHandler,
) CommandBus {
	return &commandBus{
		logger:              logger,
		acceptTransactional: acceptTransactionalH,
		queueCampaign:       queueCampaignH,
		handleCampaign:      handleCampaignH,
		handleProvider:      handleProviderH,
		processDueMsgs:      processDueMsgsH,
	}
}

func (b *commandBus) AcceptTransactionalSend(ctx context.Context, input AcceptTransactionalSendInput) (*AcceptTransactionalSendResult, error) {
	b.logger.Info("dispatching command", "command", "accept_transactional_send")
	result, err := b.acceptTransactional.Execute(ctx, input)
	if err != nil {
		b.logger.Warn("command failed", "command", "accept_transactional_send", "error", err)
		return nil, err
	}
	return result, nil
}

func (b *commandBus) QueueCampaignMessages(ctx context.Context, input QueueCampaignMessagesInput) (*QueueCampaignMessagesResult, error) {
	b.logger.Info("dispatching command", "command", "queue_campaign_messages")
	result, err := b.queueCampaign.Execute(ctx, input)
	if err != nil {
		b.logger.Warn("command failed", "command", "queue_campaign_messages", "error", err)
		return nil, err
	}
	return result, nil
}

func (b *commandBus) HandleCampaignScheduled(ctx context.Context, input HandleCampaignScheduledInput) error {
	b.logger.Info("dispatching command", "command", "handle_campaign_scheduled")
	err := b.handleCampaign.Execute(ctx, input)
	if err != nil {
		b.logger.Warn("command failed", "command", "handle_campaign_scheduled", "error", err)
	}
	return err
}

func (b *commandBus) HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error) {
	b.logger.Info("dispatching command", "command", "handle_provider_event")
	result, err := b.handleProvider.Execute(ctx, input)
	if err != nil {
		b.logger.Warn("command failed", "command", "handle_provider_event", "error", err)
		return nil, err
	}
	return result, nil
}

func (b *commandBus) ProcessDueMessagesAllWorkspaces(ctx context.Context, input ProcessDueMessagesAllInput) (int, error) {
	b.logger.Info("dispatching command", "command", "process_due_messages_all")
	result, err := b.processDueMsgs.ProcessDueMessagesAllWorkspaces(ctx, input)
	if err != nil {
		b.logger.Warn("command failed", "command", "process_due_messages_all", "error", err)
	}
	return result, err
}

func (b *commandBus) ProcessDueMessages(ctx context.Context, input ProcessDueMessagesInput) (*ProcessDueMessagesResult, error) {
	b.logger.Info("dispatching command", "command", "process_due_messages")
	result, err := b.processDueMsgs.ProcessDueMessages(ctx, input)
	if err != nil {
		b.logger.Warn("command failed", "command", "process_due_messages", "error", err)
		return nil, err
	}
	return result, nil
}

type queryBus struct {
	logger         *slog.Logger
	messageQueries *message.QueryService
}

func newQueryBus(
	logger *slog.Logger,
	messageQueries *message.QueryService,
) QueryBus {
	return &queryBus{
		logger:         logger,
		messageQueries: messageQueries,
	}
}

func (b *queryBus) ListMessages(ctx context.Context, input ListMessagesInput) (*ListMessagesResult, error) {
	b.logger.Info("dispatching query", "query", "list_messages")
	result, err := b.messageQueries.ListMessages(ctx, input)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_messages", "error", err)
		return nil, err
	}
	return result, nil
}

func (b *queryBus) GetMessage(ctx context.Context, input GetMessageInput) (*domain.Message, error) {
	b.logger.Info("dispatching query", "query", "get_message")
	result, err := b.messageQueries.GetMessage(ctx, input)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_message", "error", err)
	}
	return result, err
}

func (b *queryBus) GetTransactionalMessage(ctx context.Context, workspaceID, messageID string) (*GetTransactionalMessageResult, error) {
	b.logger.Info("dispatching query", "query", "get_transactional_message")
	result, err := b.messageQueries.GetTransactionalMessage(ctx, workspaceID, messageID)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_transactional_message", "error", err)
		return nil, err
	}
	return result, nil
}
