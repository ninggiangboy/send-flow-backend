package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/getnotificationstatus"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/listnotifications"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/processretrybatch"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/sendsystemalert"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/sendwelcomeemail"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/sendworkspaceinvitationemail"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/usecase"
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

type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

type GetNotificationStatusResult = getnotificationstatus.Result

type ListNotificationsResult = listnotifications.Result

type Service struct {
	sendWelcomeEmailH           *sendwelcomeemail.Handler
	sendWorkspaceInvitationH    *sendworkspaceinvitationemail.Handler
	sendSystemAlertH            *sendsystemalert.Handler
	processRetryBatchH          *processretrybatch.Handler
	getNotificationStatusH      *getnotificationstatus.Handler
	listNotificationsH          *listnotifications.Handler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}

	eventPub := usecase.NewEventPublisher(opts.OutboxWriter, opts.IDGen)
	emailSender := usecase.NewEmailSender(opts.MessagesWrite, opts.AttemptsWrite, opts.EmailSender, eventPub, opts.IDGen, opts.Logger)

	return &Service{
		sendWelcomeEmailH: sendwelcomeemail.New(sendwelcomeemail.Options{
			MessagesWrite: opts.MessagesWrite,
			TxManager:     opts.TxManager,
			EmailSender:   emailSender,
			EventPub:      eventPub,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		sendWorkspaceInvitationH: sendworkspaceinvitationemail.New(sendworkspaceinvitationemail.Options{
			MessagesWrite: opts.MessagesWrite,
			TxManager:     opts.TxManager,
			EmailSender:   emailSender,
			EventPub:      eventPub,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		sendSystemAlertH: sendsystemalert.New(sendsystemalert.Options{
			MessagesWrite: opts.MessagesWrite,
			TxManager:     opts.TxManager,
			EmailSender:   emailSender,
			EventPub:      eventPub,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		processRetryBatchH: processretrybatch.New(processretrybatch.Options{
			MessagesWrite: opts.MessagesWrite,
			EmailSender:   emailSender,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		getNotificationStatusH: getnotificationstatus.New(getnotificationstatus.Options{
			MessagesRead:  opts.MessagesRead,
			AttemptsRead:  opts.AttemptsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		listNotificationsH: listnotifications.New(listnotifications.Options{
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
	return s.getNotificationStatusH.Execute(ctx, getnotificationstatus.Query{
		WorkspaceID: workspaceID,
		MessageID:   messageID,
		UserID:      userID,
	})
}

func (s *Service) ListNotifications(ctx context.Context, filter domain.NotificationFilter, userID string) (*ListNotificationsResult, error) {
	return s.listNotificationsH.Execute(ctx, listnotifications.Query{
		Filter: filter,
		UserID: userID,
	})
}

// wrapError converts handler-specific non-retryable errors into app.NonRetryableError.
func wrapError(err error) error {
	var nr interface{ NonRetryable() bool }
	if errors.As(err, &nr) {
		return &NonRetryableError{Err: err}
	}
	return err
}
