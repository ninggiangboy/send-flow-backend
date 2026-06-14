package kafka

import (
	"context"
	"sync"

	segmentio "github.com/segmentio/kafka-go"
)

// WriterProducer implements Producer using a segmentio/kafka-go Writer.
type WriterProducer struct {
	writer *segmentio.Writer
	mu     sync.Mutex
	closed bool
}

// NewWriterProducer creates a new producer that writes to the given brokers.
// If brokers is empty, returns a DisabledProducer.
func NewWriterProducer(brokers []string) *WriterProducer {
	if len(brokers) == 0 {
		return &WriterProducer{}
	}
	return &WriterProducer{
		writer: &segmentio.Writer{
			Addr:     segmentio.TCP(brokers...),
			Balancer: &segmentio.Murmur2Balancer{},
		},
	}
}

// Produce sends a message to its topic.
func (p *WriterProducer) Produce(ctx context.Context, msg Message) error {
	if p.writer == nil {
		return ErrDisabled
	}

	headers := make([]segmentio.Header, 0, len(msg.Headers))
	for k, v := range msg.Headers {
		headers = append(headers, segmentio.Header{Key: k, Value: []byte(v)})
	}

	return p.writer.WriteMessages(ctx, segmentio.Message{
		Topic:   msg.Topic,
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: headers,
	})
}

// Close shuts down the underlying writer.
func (p *WriterProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.writer == nil {
		return nil
	}
	p.closed = true
	return p.writer.Close()
}
