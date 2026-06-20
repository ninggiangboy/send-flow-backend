package retry

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/batching"
)

type ProcessOptions struct {
	MessagesWrite ports.MessageWriteRepository
	EmailSender   *shared.EmailSender
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type ProcessHandler struct {
	messagesWrite ports.MessageWriteRepository
	emailSender   *shared.EmailSender
	idGen         func() (string, error)
	log           *slog.Logger
}

func NewProcess(opts ProcessOptions) *ProcessHandler {
	return &ProcessHandler{
		messagesWrite: opts.MessagesWrite,
		emailSender:   opts.EmailSender,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("shared", "process_retry_batch"),
	}
}

func (h *ProcessHandler) Execute(ctx context.Context, limit int) (int, error) {
	messages, err := h.messagesWrite.ClaimRetryingMessages(ctx, limit)
	if err != nil {
		h.log.Error("failed to claim retrying messages", "error", err)
		return 0, err
	}

	processed := 0
	pipelineLimit := limit
	if pipelineLimit <= 0 {
		pipelineLimit = len(messages)
		if pipelineLimit <= 0 {
			pipelineLimit = 1
		}
	}
	idx := 0
	pipeline, err := batching.NewPipeline[domain.NotificationMessage, struct{}](batching.Config[domain.NotificationMessage, struct{}]{
		Options: batching.Options{
			BufferedItemsSize:    pipelineLimit,
			WriteBatchSize:       pipelineLimit,
			ProcessorConcurrency: 1,
			MaxInflight:          pipelineLimit,
		},
		Reader: batching.ItemReaderFunc[domain.NotificationMessage](func(ctx context.Context, _ int, _ int) (domain.NotificationMessage, bool, error) {
			if idx >= len(messages) {
				return domain.NotificationMessage{}, false, nil
			}
			msg := messages[idx]
			idx++
			return msg, true, nil
		}),
		Processor: batching.ItemProcessorFunc[domain.NotificationMessage, struct{}](func(ctx context.Context, msg domain.NotificationMessage) (struct{}, bool, error) {
			msgCopy := msg
			attemptNumber := msg.AttemptCount + 1
			_, final, sendErr := h.emailSender.Send(ctx, &msgCopy, attemptNumber, "smtp")
			if sendErr != nil {
				h.log.Error("failed to retry notification",
					"message_id", msgCopy.ID,
					"attempt", attemptNumber,
					"error", sendErr,
				)
				return struct{}{}, false, nil
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
			return struct{}{}, true, nil
		}),
		Writer: batching.ItemWriterFunc[struct{}](func(ctx context.Context, batch []struct{}) error {
			processed += len(batch)
			return nil
		}),
	})
	if err != nil {
		return 0, err
	}
	if err := pipeline.Run(ctx); err != nil {
		return 0, err
	}

	return processed, nil
}
