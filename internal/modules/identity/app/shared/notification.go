package shared

import (
	"context"
	"net/url"
	"strings"

	emailtemplates "github.com/ninggiangboy/send-flow/backend/internal/templates/email"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type NotificationService struct {
	mailSender      ports.Mailer
	frontendBaseURL string
}

func NewNotificationService(mailSender ports.Mailer, frontendBaseURL string) *NotificationService {
	return &NotificationService{
		mailSender:      mailSender,
		frontendBaseURL: frontendBaseURL,
	}
}

func (s *NotificationService) BuildURL(path, token string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(s.frontendBaseURL), "/")
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

func (s *NotificationService) SendEmail(ctx context.Context, to, subject, text, html string) error {
	if s.mailSender == nil {
		return nil
	}
	return s.mailSender.Send(ctx, []string{to}, subject, text, html)
}

func (s *NotificationService) BuildEmailVerificationLink(token string) string {
	return s.BuildURL("/verify-email", token)
}

func (s *NotificationService) SendVerificationEmail(ctx context.Context, to, link string) error {
	subject, text, html, err := emailtemplates.RenderVerificationEmail(link)
	if err != nil {
		return err
	}
	return s.SendEmail(ctx, to, subject, text, html)
}

func (s *NotificationService) BuildPasswordResetLink(token string) string {
	return s.BuildURL("/reset-password", token)
}

func (s *NotificationService) SendPasswordResetEmail(ctx context.Context, to, link string) error {
	subject, text, html, err := emailtemplates.RenderPasswordResetEmail(link)
	if err != nil {
		return err
	}
	return s.SendEmail(ctx, to, subject, text, html)
}
