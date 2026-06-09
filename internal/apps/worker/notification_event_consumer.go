package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

type NotificationEventConsumer struct {
	name            string
	svc             *notificationapp.Service
	log             *slog.Logger
	brokers         []string
	groupID         string
	markers         *ProcessedEventMarkers
	deadLetter      *DeadLetterRepository
	idGen           func() (string, error)
	frontendBaseURL string
}

func NewNotificationEventConsumer(svc *notificationapp.Service, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool, frontendBaseURL string) *NotificationEventConsumer {
	c := &NotificationEventConsumer{
		name:            "notification.identity_events",
		svc:             svc,
		log:             log.With("consumer", "notification.identity_events"),
		brokers:         brokers,
		groupID:         groupID,
		idGen:           id.NewUUIDGenerator().New,
		frontendBaseURL: frontendBaseURL,
	}
	if pool != nil {
		c.markers = NewProcessedEventMarkers(pool)
		c.deadLetter = NewDeadLetterRepository(pool)
	}
	return c
}

func (c *NotificationEventConsumer) Name() string {
	return c.name
}

func (c *NotificationEventConsumer) Run(ctx context.Context) error {
	if len(c.brokers) == 0 {
		c.log.Info("kafka not configured, consumer disabled")
		<-ctx.Done()
		return nil
	}

	userRegisteredTopic := events.TopicFromEventType("identity.user.registered.v1")
	memberInvitedTopic := events.TopicFromEventType("identity.workspace.member_invited.v1")

	consumer, err := kafka.NewReaderConsumer(kafka.ReaderConsumerOptions{
		Brokers:     c.brokers,
		GroupTopics: []string{userRegisteredTopic, memberInvitedTopic},
		GroupID:     c.groupID,
	})
	if err != nil {
		if errors.Is(err, kafka.ErrDisabled) {
			c.log.Info("kafka disabled, consumer not starting")
			<-ctx.Done()
			return nil
		}
		return err
	}
	defer consumer.Close()

	c.log.Info("starting notification identity event consumer",
		"topics", []string{userRegisteredTopic, memberInvitedTopic},
		"group_id", c.groupID,
	)

	return consumer.Consume(ctx, func(ctx context.Context, msg kafka.Message) error {
		eventID := msg.Headers["event_id"]
		if eventID == "" {
			c.log.Warn("received message without event_id header, skipping")
			return nil
		}

		eventType := msg.Headers["event_type"]

		if c.markers != nil {
			already, err := c.markers.WasProcessed(ctx, c.name, eventID)
			if err != nil {
				c.log.Error("failed to check processed marker", "event_id", eventID, "error", err)
				return err
			}
			if already {
				c.log.Debug("duplicate event, skipping", "event_id", eventID)
				return nil
			}
		}

		var handleErr error
		switch eventType {
		case "identity.user.registered.v1":
			handleErr = c.handleUserRegisteredEvent(ctx, eventID, msg.Value)
		case "identity.workspace.member_invited.v1":
			handleErr = c.handleMemberInvitedEvent(ctx, eventID, msg.Value)
		default:
			c.log.Warn("unknown event type, skipping", "event_type", eventType, "event_id", eventID)
			if c.markers != nil {
				if _, mErr := c.markers.MarkProcessed(ctx, c.name, eventID); mErr != nil {
					return mErr
				}
			}
			return nil
		}

		if handleErr != nil {
			var nonRetryable *notificationapp.NonRetryableError
			if errors.As(handleErr, &nonRetryable) {
				c.log.Warn("non-retryable error handling notification event",
					"event_id", eventID,
					"event_type", eventType,
					"error", handleErr,
				)
				if c.deadLetter != nil {
					if dlErr := c.deadLetter.Save(ctx, DeadLetterRecord{
						ID:           mustNewID(c.idGen),
						WorkspaceID:  workspaceIDFromMessage(msg.Headers, msg.Value),
						Source:       c.name,
						EventID:      eventID,
						Payload:      msg.Value,
						ErrorMessage: handleErr.Error(),
						Retryable:    false,
					}); dlErr != nil {
						c.log.Error("failed to save dead letter record, returning for retry",
							"event_id", eventID, "error", dlErr,
						)
						return dlErr
					}
				}
				if c.markers != nil {
					if _, mErr := c.markers.MarkProcessed(ctx, c.name, eventID); mErr != nil {
						return mErr
					}
				}
				return nil
			}

			c.log.Error("retryable error handling notification event",
				"event_id", eventID,
				"event_type", eventType,
				"error", handleErr,
			)
			return handleErr
		}

		if c.markers != nil {
			if _, err := c.markers.MarkProcessed(ctx, c.name, eventID); err != nil {
				c.log.Error("failed to mark event as processed, returning for retry",
					"event_id", eventID, "error", err,
				)
				return err
			}
		}
		return nil
	})
}

type userRegisteredIdentityPayload struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	AuthMethod string `json:"auth_method"`
	At         string `json:"at"`
}

type memberInvitedIdentityPayload struct {
	WorkspaceID     string `json:"workspace_id"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	InvitedBy       string `json:"invited_by"`
	InvitedByEmail  string `json:"invited_by_email"`
	At              string `json:"at"`
}

func (c *NotificationEventConsumer) handleUserRegisteredEvent(ctx context.Context, eventID string, payload []byte) error {
	var env events.Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		c.log.Warn("failed to unmarshal user registered event envelope", "event_id", eventID, "error", err)
		return &notificationapp.NonRetryableError{Err: err}
	}

	var evt userRegisteredIdentityPayload
	if err := json.Unmarshal(env.Payload, &evt); err != nil {
		c.log.Warn("failed to unmarshal user registered event payload", "event_id", eventID, "error", err)
		return &notificationapp.NonRetryableError{Err: err}
	}

	c.log.Info("processing user registered event",
		"event_id", eventID,
		"user_id", evt.UserID,
		"email", evt.Email,
	)

	_, err := c.svc.SendWelcomeEmail(ctx, domain.SendWelcomeEmailInput{
		UserID:          evt.UserID,
		Email:           evt.Email,
		FrontendBaseURL: c.frontendBaseURL,
	})

	return err
}

func (c *NotificationEventConsumer) handleMemberInvitedEvent(ctx context.Context, eventID string, payload []byte) error {
	var env events.Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		c.log.Warn("failed to unmarshal member invited event envelope", "event_id", eventID, "error", err)
		return &notificationapp.NonRetryableError{Err: err}
	}

	var evt memberInvitedIdentityPayload
	if err := json.Unmarshal(env.Payload, &evt); err != nil {
		c.log.Warn("failed to unmarshal member invited event payload", "event_id", eventID, "error", err)
		return &notificationapp.NonRetryableError{Err: err}
	}

	c.log.Info("processing member invited event",
		"event_id", eventID,
		"workspace_id", evt.WorkspaceID,
		"email", evt.Email,
	)

	_, err := c.svc.SendWorkspaceInvitationEmail(ctx, domain.SendInvitationEmailInput{
		WorkspaceID:     evt.WorkspaceID,
		InvitedByEmail:  evt.InvitedByEmail,
		InvitedByUserID: evt.InvitedBy,
		InviteeEmail:    evt.Email,
		Role:            evt.Role,
		FrontendBaseURL: c.frontendBaseURL,
	})

	return err
}
