package app

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/accepttransactionalsend"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/getmessage"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/gettransactionalmessage"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/handlecampaignscheduled"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/handleproviderevent"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/listmessages"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/processduemessages"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/queuecampaignmessages"
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
	acceptTransactional *accepttransactionalsend.Handler
	queueCampaign       *queuecampaignmessages.Handler
	handleCampaign      *handlecampaignscheduled.Handler
	handleProvider      *handleproviderevent.Handler
	processDueMsgs      *processduemessages.Handler
}

func newCommandBus(
	logger *slog.Logger,
	acceptTransactionalH *accepttransactionalsend.Handler,
	queueCampaignH *queuecampaignmessages.Handler,
	handleCampaignH *handlecampaignscheduled.Handler,
	handleProviderH *handleproviderevent.Handler,
	processDueMsgsH *processduemessages.Handler,
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
	result, err := b.acceptTransactional.Execute(ctx, accepttransactionalsend.Input{
		WorkspaceID:       input.WorkspaceID,
		APIKeyID:          input.APIKeyID,
		IdempotencyKey:    input.IdempotencyKey,
		Mode:              input.Mode,
		SenderDomainID:    input.SenderDomainID,
		SenderName:        input.SenderName,
		Subject:           input.Subject,
		TemplateID:        input.TemplateID,
		TemplateVersionID: input.TemplateVersionID,
		TemplateData:      input.TemplateData,
		TextBody:          input.TextBody,
		HTMLBody:          input.HTMLBody,
		ReplyTo:           input.ReplyTo,
		To:                input.To,
		CC:                input.CC,
		BCC:               input.BCC,
		Metadata:          input.Metadata,
		Tags:              input.Tags,
		Headers:           input.Headers,
		Attachments:       input.Attachments,
		Now:               input.Now,
	})
	if err != nil {
		b.logger.Warn("command failed", "command", "accept_transactional_send", "error", err)
		return nil, err
	}
	return &AcceptTransactionalSendResult{
		RequestID:  result.RequestID,
		MessageIDs: result.MessageIDs,
		Status:     result.Status,
		AcceptedAt: result.AcceptedAt,
	}, nil
}

func (b *commandBus) QueueCampaignMessages(ctx context.Context, input QueueCampaignMessagesInput) (*QueueCampaignMessagesResult, error) {
	b.logger.Info("dispatching command", "command", "queue_campaign_messages")
	result, err := b.queueCampaign.Execute(ctx, queuecampaignmessages.Input{
		WorkspaceID:       input.WorkspaceID,
		CampaignID:        input.CampaignID,
		TemplateID:        input.TemplateID,
		TemplateVersionID: input.TemplateVersionID,
		SenderDomainID:    input.SenderDomainID,
		MessageType:       input.MessageType,
		ScheduledAt:       input.ScheduledAt,
		Now:               input.Now,
	})
	if err != nil {
		b.logger.Warn("command failed", "command", "queue_campaign_messages", "error", err)
		return nil, err
	}
	return &QueueCampaignMessagesResult{
		QueuedCount:    result.QueuedCount,
		CandidateCount: result.CandidateCount,
	}, nil
}

func (b *commandBus) HandleCampaignScheduled(ctx context.Context, input HandleCampaignScheduledInput) error {
	b.logger.Info("dispatching command", "command", "handle_campaign_scheduled")
	err := b.handleCampaign.Execute(ctx, handlecampaignscheduled.Input{
		EventID:   input.EventID,
		EventType: input.EventType,
		Payload:   input.Payload,
		Now:       input.Now,
	})
	if err != nil {
		b.logger.Warn("command failed", "command", "handle_campaign_scheduled", "error", err)
	}
	return err
}

func (b *commandBus) HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error) {
	b.logger.Info("dispatching command", "command", "handle_provider_event")
	result, err := b.handleProvider.Execute(ctx, handleproviderevent.Input{
		EventID:           input.EventID,
		NormalizedEventID: input.NormalizedEventID,
		RawEventID:        input.RawEventID,
		WorkspaceID:       input.WorkspaceID,
		MessageID:         input.MessageID,
		Provider:          input.Provider,
		ProviderEventID:   input.ProviderEventID,
		ProviderMessageID: input.ProviderMessageID,
		EventType:         input.EventType,
		OccurredAt:        input.OccurredAt,
		ReceivedAt:        input.ReceivedAt,
	})
	if err != nil {
		b.logger.Warn("command failed", "command", "handle_provider_event", "error", err)
		return nil, err
	}
	return &HandleProviderEventResult{
		Handled:            result.Handled,
		Ignored:            result.Ignored,
		MessageID:          result.MessageID,
		WorkspaceID:        result.WorkspaceID,
		PreviousStatus:     result.PreviousStatus,
		NewStatus:          result.NewStatus,
		SuppressionCreated: result.SuppressionCreated,
		SuppressionEntryID: result.SuppressionEntryID,
	}, nil
}

func (b *commandBus) ProcessDueMessagesAllWorkspaces(ctx context.Context, input ProcessDueMessagesAllInput) (int, error) {
	b.logger.Info("dispatching command", "command", "process_due_messages_all")
	result, err := b.processDueMsgs.ProcessDueMessagesAllWorkspaces(ctx, processduemessages.ProcessDueMessagesAllInput{
		MessageType: input.MessageType,
		Limit:       input.Limit,
		Now:         input.Now,
	})
	if err != nil {
		b.logger.Warn("command failed", "command", "process_due_messages_all", "error", err)
	}
	return result, err
}

func (b *commandBus) ProcessDueMessages(ctx context.Context, input ProcessDueMessagesInput) (*ProcessDueMessagesResult, error) {
	b.logger.Info("dispatching command", "command", "process_due_messages")
	result, err := b.processDueMsgs.ProcessDueMessages(ctx, processduemessages.ProcessDueMessagesInput{
		WorkspaceID: input.WorkspaceID,
		MessageType: input.MessageType,
		Limit:       input.Limit,
		Now:         input.Now,
	})
	if err != nil {
		b.logger.Warn("command failed", "command", "process_due_messages", "error", err)
		return nil, err
	}
	return &ProcessDueMessagesResult{
		SelectedCount:       result.SelectedCount,
		AcceptedCount:       result.AcceptedCount,
		FailedCount:         result.FailedCount,
		RetryScheduledCount: result.RetryScheduledCount,
	}, nil
}

type queryBus struct {
	logger           *slog.Logger
	listMessages     *listmessages.Handler
	getMessage       *getmessage.Handler
	getTransactional *gettransactionalmessage.Handler
}

func newQueryBus(
	logger *slog.Logger,
	listMessagesH *listmessages.Handler,
	getMessageH *getmessage.Handler,
	getTransactionalH *gettransactionalmessage.Handler,
) QueryBus {
	return &queryBus{
		logger:           logger,
		listMessages:     listMessagesH,
		getMessage:       getMessageH,
		getTransactional: getTransactionalH,
	}
}

func (b *queryBus) ListMessages(ctx context.Context, input ListMessagesInput) (*ListMessagesResult, error) {
	b.logger.Info("dispatching query", "query", "list_messages")
	result, err := b.listMessages.Execute(ctx, listmessages.Input{
		UserID:                   input.UserID,
		WorkspaceID:              input.WorkspaceID,
		CampaignID:               input.CampaignID,
		TransactionalRequestID:   input.TransactionalRequestID,
		Status:                   input.Status,
		MessageType:              input.MessageType,
		Mode:                     input.Mode,
		Provider:                 input.Provider,
		RecipientEmailNormalized: input.RecipientEmailNormalized,
		ProviderMessageID:        input.ProviderMessageID,
		From:                     input.From,
		To:                       input.To,
		Limit:                    input.Limit,
		Cursor:                   input.Cursor,
	})
	if err != nil {
		b.logger.Warn("query failed", "query", "list_messages", "error", err)
		return nil, err
	}
	return &ListMessagesResult{Messages: result.Messages, NextCursor: result.NextCursor}, nil
}

func (b *queryBus) GetMessage(ctx context.Context, input GetMessageInput) (*domain.Message, error) {
	b.logger.Info("dispatching query", "query", "get_message")
	result, err := b.getMessage.Execute(ctx, getmessage.Input{
		UserID:      input.UserID,
		WorkspaceID: input.WorkspaceID,
		MessageID:   input.MessageID,
	})
	if err != nil {
		b.logger.Warn("query failed", "query", "get_message", "error", err)
	}
	return result, err
}

func (b *queryBus) GetTransactionalMessage(ctx context.Context, workspaceID, messageID string) (*GetTransactionalMessageResult, error) {
	b.logger.Info("dispatching query", "query", "get_transactional_message")
	result, err := b.getTransactional.Execute(ctx, workspaceID, messageID)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_transactional_message", "error", err)
		return nil, err
	}
	return &GetTransactionalMessageResult{
		MessageID:         result.MessageID,
		Status:            result.Status,
		Provider:          result.Provider,
		ProviderMessageID: result.ProviderMessageID,
		LastUpdatedAt:     result.LastUpdatedAt,
	}, nil
}
