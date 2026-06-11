package usecase

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func EmitEvent(ctx context.Context, outbox ports.OutboxWriter, idGen ports.IDGenerator, eventType, aggregateType, aggregateID, workspaceID string, payload any, now time.Time) error {
	if outbox == nil {
		return nil
	}
	eventID, err := idGen.New()
	if err != nil {
		return err
	}
	env, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     eventType,
		EventVersion:  1,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		WorkspaceID:   workspaceID,
		OccurredAt:    now,
	}, payload)
	if err != nil {
		return err
	}
	payloadBytes, err := events.Marshal(env)
	if err != nil {
		return err
	}
	return outbox.Save(ctx, ports.OutboxEvent{
		ID:            eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadBytes,
		WorkspaceID:   workspaceID,
		OccurredAt:    now,
	})
}

func CreateAuthToken(ctx context.Context, deps Deps, userID, purpose string, ttl time.Duration, now time.Time) (string, error) {
	if deps.AuthTokens == nil {
		return "", nil
	}
	if err := deps.AuthTokens.DeleteByUserAndPurpose(ctx, userID, purpose); err != nil {
		return "", err
	}
	raw, err := deps.TokenGen.RandomToken(32)
	if err != nil {
		return "", err
	}
	id, err := deps.IDGen.New()
	if err != nil {
		return "", err
	}
	token := domain.AuthToken{
		ID:        id,
		UserID:    userID,
		Purpose:   purpose,
		TokenHash: deps.TokenHasher.HashToken(raw),
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	if err := deps.AuthTokens.Create(ctx, token); err != nil {
		return "", err
	}
	return raw, nil
}

func ConsumeAuthToken(ctx context.Context, deps Deps, raw, purpose string, now time.Time) (*domain.AuthToken, error) {
	if deps.AuthTokens == nil || strings.TrimSpace(raw) == "" {
		return nil, domain.ErrUnauthorized
	}
	token, err := deps.AuthTokens.FindByHash(ctx, purpose, deps.TokenHasher.HashToken(raw))
	if err != nil {
		return nil, err
	}
	if !token.IsUsable(now) {
		return nil, domain.ErrUnauthorized
	}
	if err := deps.AuthTokens.Consume(ctx, token.ID, now); err != nil {
		return nil, err
	}
	return token, nil
}

func BuildFrontendURL(baseURL, path, token string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return token
	}
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return token
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}

func SendBasicEmail(ctx context.Context, deps Deps, to, subject, body string) error {
	if deps.MailSender == nil {
		return nil
	}
	return deps.MailSender.Send(ctx, []string{to}, subject, body, body)
}

func VerificationEmailBody(link string) string {
	return fmt.Sprintf("Verify your email by opening this link: %s", link)
}

func PasswordResetEmailBody(link string) string {
	return fmt.Sprintf("Reset your password by opening this link: %s", link)
}

func CreateEmailVerificationToken(ctx context.Context, deps Deps, user domain.User, now time.Time) (string, error) {
	return CreateAuthToken(ctx, deps, user.ID, domain.AuthTokenPurposeEmailVerification, deps.VerificationTTL, now)
}

func BuildEmailVerificationLink(deps Deps, token string) string {
	return BuildFrontendURL(deps.FrontendBaseURL, "/verify-email", token)
}

func SendVerificationEmail(ctx context.Context, deps Deps, to, link string) error {
	return SendBasicEmail(ctx, deps, to, "Verify your email", VerificationEmailBody(link))
}

func CreatePasswordResetToken(ctx context.Context, deps Deps, userID string, now time.Time) (string, error) {
	return CreateAuthToken(ctx, deps, userID, domain.AuthTokenPurposePasswordReset, deps.PasswordResetTTL, now)
}

func BuildPasswordResetLink(deps Deps, token string) string {
	return BuildFrontendURL(deps.FrontendBaseURL, "/reset-password", token)
}

func SendPasswordResetEmail(ctx context.Context, deps Deps, to, link string) error {
	return SendBasicEmail(ctx, deps, to, "Reset your password", PasswordResetEmailBody(link))
}

func CreateMFAChallenge(ctx context.Context, deps Deps, userID string, now time.Time) (string, error) {
	return CreateAuthToken(ctx, deps, userID, domain.AuthTokenPurposeMFAChallenge, deps.MFAChallengeTTL, now)
}
