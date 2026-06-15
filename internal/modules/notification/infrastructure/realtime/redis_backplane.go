package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
)

const (
	workspaceChannelPrefix = "sendflow:notifications:workspace:"
	userChannelPrefix      = "sendflow:notifications:user:"
)

type Backplane struct {
	client goredis.UniversalClient
	log    *slog.Logger
}

func NewBackplane(client goredis.UniversalClient, log *slog.Logger) *Backplane {
	return &Backplane{client: client, log: log}
}

func WorkspaceChannel(workspaceID string) string {
	return workspaceChannelPrefix + workspaceID
}

func UserChannel(userID string) string {
	return userChannelPrefix + userID
}

func (b *Backplane) Publish(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal realtime event: %w", err)
	}

	channels := make([]string, 0, 2)
	if event.WorkspaceID != "" {
		channels = append(channels, WorkspaceChannel(event.WorkspaceID))
	}
	if event.RecipientUserID != "" {
		channels = append(channels, UserChannel(event.RecipientUserID))
	}
	if len(channels) == 0 {
		b.log.Warn("realtime event has no workspace or user target, dropping",
			"event_id", event.EventID, "event_type", event.EventType)
		return nil
	}

	for _, ch := range channels {
		if err := b.client.Publish(ctx, ch, data).Err(); err != nil {
			return fmt.Errorf("publish to %s: %w", ch, err)
		}
	}
	return nil
}

func (b *Backplane) Subscribe(ctx context.Context, channel string) (<-chan Event, func() error, error) {
	pubsub := b.client.Subscribe(ctx, channel)
	ch := make(chan Event, 32)

	go func() {
		defer func() {
			pubsub.Close()
			close(ch)
		}()

		msgCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				var ev Event
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					b.log.Error("malformed realtime event from redis",
						"channel", channel, "error", err)
					continue
				}
				select {
				case ch <- ev:
				default:
					b.log.Warn("realtime subscriber buffer full, dropping event",
						"channel", channel, "event_id", ev.EventID)
				}
			}
		}
	}()

	return ch, pubsub.Close, nil
}
