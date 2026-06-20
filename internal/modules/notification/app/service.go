package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/message"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/retry"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/send"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type Options struct {
	MessagesRead  ports.MessageReadRepository
	MessagesWrite ports.MessageWriteRepository
	AttemptsRead  ports.AttemptReadRepository
	AttemptsWrite ports.AttemptWriteRepository
	OutboxWriter  ports.OutboxWriter
	TxManager     ports.TransactionManager
	AccessChecker ports.WorkspaceAccessChecker
	EmailSender   ports.EmailSender
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type NonRetryableError = shared.NonRetryableError

type GetNotificationStatusResult = message.StatusResult

type ListNotificationsResult = message.ListResult

type Service struct {
	sendWelcomeEmailH        *send.WelcomeHandler
	sendWorkspaceInvitationH *send.InvitationHandler
	sendSystemAlertH         *send.AlertHandler
	processRetryBatchH       *retry.ProcessHandler
	getNotificationStatusH   *message.StatusHandler
	listNotificationsH       *message.ListHandler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}

	eventPub := shared.NewEventPublisher(opts.OutboxWriter, opts.IDGen)
	emailSender := shared.NewEmailSender(opts.MessagesWrite, opts.AttemptsWrite, opts.EmailSender, eventPub, opts.IDGen, opts.Logger)

	return &Service{
		sendWelcomeEmailH: send.NewWelcome(send.WelcomeOptions{
			MessagesWrite: opts.MessagesWrite,
			TxManager:     opts.TxManager,
			EmailSender:   emailSender,
			EventPub:      eventPub,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		sendWorkspaceInvitationH: send.NewInvitation(send.InvitationOptions{
			MessagesWrite: opts.MessagesWrite,
			TxManager:     opts.TxManager,
			EmailSender:   emailSender,
			EventPub:      eventPub,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		sendSystemAlertH: send.NewAlert(send.AlertOptions{
			MessagesWrite: opts.MessagesWrite,
			TxManager:     opts.TxManager,
			EmailSender:   emailSender,
			EventPub:      eventPub,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		processRetryBatchH: retry.NewProcess(retry.ProcessOptions{
			MessagesWrite: opts.MessagesWrite,
			EmailSender:   emailSender,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		getNotificationStatusH: message.NewStatus(message.StatusOptions{
			MessagesRead:  opts.MessagesRead,
			AttemptsRead:  opts.AttemptsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		listNotificationsH: message.NewList(message.ListOptions{
			MessagesRead:  opts.MessagesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
	}
}

func (s *Service) SendWelcomeEmail(ctx context.Context, input domain.SendWelcomeEmailInput) (*domain.NotificationMessage, error) {
	msg, err := s.sendWelcomeEmailH.Execute(ctx, input)
	if err != nil {
		return nil, wrapError(err)
	}
	return msg, nil
}

func (s *Service) SendWorkspaceInvitationEmail(ctx context.Context, input domain.SendInvitationEmailInput) (*domain.NotificationMessage, error) {
	msg, err := s.sendWorkspaceInvitationH.Execute(ctx, input)
	if err != nil {
		return nil, wrapError(err)
	}
	return msg, nil
}

func (s *Service) SendSystemAlert(ctx context.Context, input domain.SendSystemAlertInput) (*domain.NotificationMessage, error) {
	msg, err := s.sendSystemAlertH.Execute(ctx, input)
	if err != nil {
		return nil, wrapError(err)
	}
	return msg, nil
}

func (s *Service) ProcessRetryBatch(ctx context.Context, limit int) (int, error) {
	return s.processRetryBatchH.Execute(ctx, limit)
}

func (s *Service) GetNotificationStatus(ctx context.Context, workspaceID, messageID, userID string) (*GetNotificationStatusResult, error) {
	return s.getNotificationStatusH.Execute(ctx, message.StatusQuery{
		WorkspaceID: workspaceID,
		MessageID:   messageID,
		UserID:      userID,
	})
}

func (s *Service) ListNotifications(ctx context.Context, filter domain.NotificationFilter, userID string) (*ListNotificationsResult, error) {
	return s.listNotificationsH.Execute(ctx, message.ListQuery{
		Filter: filter,
		UserID: userID,
	})
}

// wrapError converts handler-specific non-retryable errors into app.NonRetryableError.
func wrapError(err error) error {
	var nr interface{ NonRetryable() bool }
	if errors.As(err, &nr) {
		return &shared.NonRetryableError{Err: err}
	}
	return err
}
