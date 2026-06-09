package kafka

import (
	"context"
	"errors"
	"time"

	segmentio "github.com/segmentio/kafka-go"
)

type ReaderConsumerOptions struct {
	Brokers     []string
	Topic       string
	GroupTopics []string
	GroupID     string
	MinBytes    int
	MaxBytes    int
	MaxWait     time.Duration
}

type ReaderConsumer struct {
	reader *segmentio.Reader
}

func NewReaderConsumer(opts ReaderConsumerOptions) (*ReaderConsumer, error) {
	if len(opts.Brokers) == 0 {
		return nil, ErrDisabled
	}
	if opts.Topic == "" && len(opts.GroupTopics) == 0 {
		return nil, errors.New("kafka topic or group topics is required")
	}
	if opts.GroupID == "" {
		return nil, errors.New("kafka consumer group id is required")
	}
	minBytes := opts.MinBytes
	if minBytes <= 0 {
		minBytes = 1
	}
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 10e6
	}
	maxWait := opts.MaxWait
	if maxWait <= 0 {
		maxWait = time.Second
	}

	return &ReaderConsumer{
		reader: segmentio.NewReader(segmentio.ReaderConfig{
			Brokers:     opts.Brokers,
			Topic:       opts.Topic,
			GroupTopics: opts.GroupTopics,
			GroupID:     opts.GroupID,
			MinBytes:    minBytes,
			MaxBytes:    maxBytes,
			MaxWait:     maxWait,
		}),
	}, nil
}

func (c *ReaderConsumer) Consume(ctx context.Context, handler Handler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}

		if err := handler(ctx, fromSegmentMessage(msg)); err != nil {
			return err
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *ReaderConsumer) Close() error {
	return c.reader.Close()
}

func fromSegmentMessage(msg segmentio.Message) Message {
	headers := make(map[string]string, len(msg.Headers))
	for _, header := range msg.Headers {
		headers[header.Key] = string(header.Value)
	}
	return Message{
		Topic:   msg.Topic,
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: headers,
	}
}
