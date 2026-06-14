package processretrybatch

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

type Options struct {
	MessagesWrite ports.MessageWriteRepository
	EmailSender   *usecase.EmailSender
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type Handler struct {
	messagesWrite ports.MessageWriteRepository
	emailSender   *usecase.EmailSender
	idGen         func() (string, error)
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		messagesWrite: opts.MessagesWrite,
		emailSender:   opts.EmailSender,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("usecase", "process_retry_batch"),
	}
}

func (h *Handler) Execute(ctx context.Context, limit int) (int, error) {
	messages, err := h.messagesWrite.ClaimRetryingMessages(ctx, limit)
	if err != nil {
		h.log.Error("failed to claim retrying messages", "error", err)
		return 0, err
	}

	processed := 0
	for _, msg := range messages {
		msgCopy := msg
		attemptNumber := msg.AttemptCount + 1
		_, final, sendErr := h.emailSender.Send(ctx, &msgCopy, attemptNumber, "smtp")
		if sendErr != nil {
			h.log.Error("failed to retry notification",
				"message_id", msgCopy.ID,
				"attempt", attemptNumber,
				"error", sendErr,
			)
			continue
		}
		if final {
			h.log.Info("notification permanently failed after retry",
				"message_id", msgCopy.ID,
				"attempts", msgCopy.AttemptCount,
			)
		} else {
			h.log.Info("notification retry succeeded",
				"message_id", msgCopy.ID,
				"attempt", attemptNumber,
			)
		}
		processed++
	}

	return processed, nil
}
