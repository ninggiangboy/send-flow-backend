package message

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	platformconstants "github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

// --- ListMessages ---

type ListMessagesInput struct {
	UserID                   string
	WorkspaceID              string
	CampaignID               string
	TransactionalRequestID   string
	Status                   string
	MessageType              string
	Mode                     string
	Provider                 string
	RecipientEmailNormalized string
	ProviderMessageID        string
	From                     *time.Time
	To                       *time.Time
	Limit                    int
	Cursor                   string
}

type ListMessagesResult struct {
	Messages   []domain.Message
	NextCursor string
}

// --- GetMessage ---

type GetMessageInput struct {
	UserID      string
	WorkspaceID string
	MessageID   string
}

// --- GetTransactionalMessage ---

type GetTransactionalMessageInput struct {
	WorkspaceID string
	MessageID   string
}

type GetTransactionalMessageResult struct {
	MessageID         string     `json:"message_id"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider,omitempty"`
	ProviderMessageID string     `json:"provider_message_id,omitempty"`
	LastUpdatedAt     *time.Time `json:"last_updated_at"`
}

// QueryService consolidates all delivery read queries.
type QueryService struct {
	messagesRead  ports.MessageReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewQueryService(
	messagesRead ports.MessageReadRepository,
	accessChecker ports.WorkspaceAccessChecker,
	logger *slog.Logger,
) *QueryService {
	return &QueryService{
		messagesRead:  messagesRead,
		accessChecker: accessChecker,
		log:           logger.With("module", "delivery_message_query"),
	}
}

func (s *QueryService) ListMessages(ctx context.Context, input ListMessagesInput) (*ListMessagesResult, error) {
	if input.WorkspaceID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, platformconstants.PermissionDeliveryRead); err != nil {
			return nil, err
		}
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = platformconstants.DefaultPageSize
	}

	emailNormalized := strings.TrimSpace(strings.ToLower(input.RecipientEmailNormalized))

	query := ports.MessageListQuery{
		WorkspaceID:              input.WorkspaceID,
		CampaignID:               input.CampaignID,
		TransactionalRequestID:   input.TransactionalRequestID,
		Status:                   input.Status,
		MessageType:              input.MessageType,
		Mode:                     input.Mode,
		Provider:                 input.Provider,
		RecipientEmailNormalized: emailNormalized,
		ProviderMessageID:        input.ProviderMessageID,
		From:                     input.From,
		To:                       input.To,
		Limit:                    limit,
		Cursor:                   input.Cursor,
	}

	messages, cursor, err := s.messagesRead.List(ctx, query)
	if err != nil {
		s.log.Error("failed to list messages", "workspace_id", input.WorkspaceID, "error", err)
		return nil, err
	}

	return &ListMessagesResult{Messages: messages, NextCursor: cursor}, nil
}

func (s *QueryService) GetMessage(ctx context.Context, input GetMessageInput) (*domain.Message, error) {
	if input.WorkspaceID == "" || input.MessageID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, platformconstants.PermissionDeliveryRead); err != nil {
			return nil, err
		}
	}

	msg, err := s.messagesRead.FindByID(ctx, input.WorkspaceID, input.MessageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			return nil, err
		}
		s.log.Error("failed to find message", "workspace_id", input.WorkspaceID, "message_id", input.MessageID, "error", err)
		return nil, err
	}

	return msg, nil
}

func (s *QueryService) GetTransactionalMessage(ctx context.Context, workspaceID, messageID string) (*GetTransactionalMessageResult, error) {
	if workspaceID == "" || messageID == "" {
		return nil, domain.ErrPayloadInvalid
	}

	msg, err := s.messagesRead.FindByID(ctx, workspaceID, messageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			return nil, err
		}
		s.log.Error("failed to find transactional message", "workspace_id", workspaceID, "message_id", messageID, "error", err)
		return nil, err
	}

	if msg.SourceType != domain.MessageSourceTransactional {
		return nil, domain.ErrMessageNotFound
	}

	ts := msg.UpdatedAt
	if msg.AcceptedAt != nil {
		ts = *msg.AcceptedAt
	}

	return &GetTransactionalMessageResult{
		MessageID:         msg.ID,
		Status:            msg.Status,
		Provider:          msg.Provider,
		ProviderMessageID: msg.ProviderMessageID,
		LastUpdatedAt:     &ts,
	}, nil
}
