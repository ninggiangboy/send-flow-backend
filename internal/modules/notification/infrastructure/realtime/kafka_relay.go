package realtime

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

type KafkaRelay struct {
	backplane *Backplane
	log       *slog.Logger

	consumer kafka.Consumer
	healthy  bool
}

func NewKafkaRelay(brokers []string, groupID string, backplane *Backplane, log *slog.Logger) (*KafkaRelay, error) {
	if len(brokers) == 0 {
		return &KafkaRelay{healthy: false}, kafka.ErrDisabled
	}

	topics := []string{
		events.TopicFromEventType(contracts.EventMessageQueuedV1),
		events.TopicFromEventType(contracts.EventMessageSentV1),
		events.TopicFromEventType(contracts.EventMessageFailedV1),
	}

	consumer, err := kafka.NewReaderConsumer(kafka.ReaderConsumerOptions{
		Brokers:     brokers,
		GroupTopics: topics,
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &KafkaRelay{
		backplane: backplane,
		log:       log.With("component", "kafka_relay"),
		consumer:  consumer,
		healthy:   true,
	}, nil
}

func (r *KafkaRelay) Run(ctx context.Context) error {
	if !r.healthy {
		return kafka.ErrDisabled
	}

	return r.consumer.Consume(ctx, func(ctx context.Context, msg kafka.Message) error {
		env, err := events.Unmarshal(msg.Value)
		if err != nil {
			r.log.Error("failed to unmarshal event envelope",
				"topic", msg.Topic, "error", err)
			return nil
		}

		ev, ok, err := NormalizeEnvelope(env)
		if err != nil {
			r.log.Error("failed to normalize event",
				"event_id", env.EventID, "event_type", env.EventType, "error", err)
			return nil
		}
		if !ok {
			return nil
		}

		if err := r.backplane.Publish(ctx, ev); err != nil {
			return err
		}

		return nil
	})
}

func (r *KafkaRelay) Healthy() bool {
	return r.healthy
}

var ErrKafkaDisabled = kafka.ErrDisabled

func IsKafkaDisabled(err error) bool {
	return errors.Is(err, kafka.ErrDisabled)
}
