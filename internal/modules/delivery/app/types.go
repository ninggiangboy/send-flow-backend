package app

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/campaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/message"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/process"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/send"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

// Type aliases for backward compatibility with external consumers (API, worker, tests).
// These allow grouped-package types to be accessed via deliveryapp.X without import changes.

// Shared
type NonRetryableError = shared.NonRetryableError

// Send (transactional)
type AcceptTransactionalSendInput = send.Input
type AcceptTransactionalSendResult = send.Result
type AttachmentStream = send.AttachmentStream

// Message queries
type ListMessagesInput = message.ListMessagesInput
type ListMessagesResult = message.ListMessagesResult
type GetMessageInput = message.GetMessageInput
type GetTransactionalMessageInput = message.GetTransactionalMessageInput
type GetTransactionalMessageResult = message.GetTransactionalMessageResult

// Campaign
type QueueCampaignMessagesInput = campaign.QueueCampaignMessagesInput
type QueueCampaignMessagesResult = campaign.QueueCampaignMessagesResult
type HandleCampaignScheduledInput = campaign.CampaignScheduledInput

// Process
type HandleProviderEventInput = process.ProviderEventInput
type HandleProviderEventResult = process.ProviderEventResult
type ProcessDueMessagesInput = process.ProcessDueMessagesInput
type ProcessDueMessagesAllInput = process.ProcessDueMessagesAllInput
type ProcessDueMessagesResult = process.ProcessDueMessagesResult

// Process - RecipientSuppressor types (used by worker adapters)
type RecipientSuppressor = process.RecipientSuppressor
type SuppressFromSignalInput = process.SuppressFromSignalInput
type SuppressFromSignalResult = process.SuppressFromSignalResult

// Service-level input/output types — used directly by service.go and external consumers
// These are not moved to grouped packages because their logic stays in the facade.

type ListMessageEventsInput struct {
	WorkspaceID string
	MessageID   string
	Limit       int
	Cursor      string
}

type ListMessageEventsResult struct {
	Events     []domain.MessageEvent
	NextCursor string
}

type ListRequestMessagesInput struct {
	WorkspaceID string
	RequestID   string
	Limit       int
	Cursor      string
}

type ListRequestMessagesResult struct {
	Messages []domain.Message
}

type ListAttemptsInput struct {
	WorkspaceID string
	MessageID   string
}
