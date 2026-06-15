package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
)

type Service struct {
	Relay *KafkaRelay
	Hub   *Hub
}

func NewService(relay *KafkaRelay, hub *Hub) *Service {
	return &Service{Relay: relay, Hub: hub}
}

func (s *Service) Run(ctx context.Context) error {
	if s.Relay == nil || !s.Relay.Healthy() {
		return errors.New("kafka relay is not available")
	}
	return s.Relay.Run(ctx)
}

func (s *Service) Healthy() bool {
	if s.Relay == nil || !s.Relay.Healthy() {
		return false
	}
	if s.Hub == nil || !s.Hub.Healthy() {
		return false
	}
	return true
}

func (s *Service) SubscribeWorkspace(ctx context.Context, workspaceID string) (<-chan Event, func(), error) {
	if workspaceID == "" {
		return nil, nil, fmt.Errorf("workspace_id is required")
	}
	if !s.Healthy() {
		return nil, nil, errors.New("realtime notification service is unavailable")
	}
	ch, unsub := s.Hub.Subscribe(ctx, WorkspaceChannel(workspaceID))
	return ch, unsub, nil
}

func (s *Service) SubscribeUser(ctx context.Context, userID string) (<-chan Event, func(), error) {
	if userID == "" {
		return nil, nil, fmt.Errorf("user_id is required")
	}
	if !s.Healthy() {
		return nil, nil, errors.New("realtime notification service is unavailable")
	}
	ch, unsub := s.Hub.Subscribe(ctx, UserChannel(userID))
	return ch, unsub, nil
}

// NewRelayFromClient is a convenience constructor that creates a KafkaRelay
// from a go-redis UniversalClient (for use in the API composition root).
func NewRelayFromClient(brokers []string, groupID string, client goredis.UniversalClient, log *slog.Logger) (*KafkaRelay, error) {
	backplane := NewBackplane(client, log)
	relay, err := NewKafkaRelay(brokers, groupID, backplane, log)
	if err != nil && !IsKafkaDisabled(err) {
		return nil, err
	}
	return relay, nil
}
