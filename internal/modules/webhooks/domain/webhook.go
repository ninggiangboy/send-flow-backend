package domain

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/httpheaders"
)

type ConfigStatus string

const (
	ConfigStatusActive   ConfigStatus = "active"
	ConfigStatusDisabled ConfigStatus = "disabled"
)

type DeliveryStatus string

const (
	DeliveryStatusPending        DeliveryStatus = "pending"
	DeliveryStatusDelivering     DeliveryStatus = "delivering"
	DeliveryStatusSucceeded      DeliveryStatus = "succeeded"
	DeliveryStatusFailed         DeliveryStatus = "failed"
	DeliveryStatusRetryScheduled DeliveryStatus = "retry_scheduled"
)

var AllowedSubscriptionEvents = map[string]bool{
	"delivery.message.queued.v1":          true,
	"delivery.message.accepted.v1":        true,
	"delivery.message.delivered.v1":       true,
	"delivery.message.bounced.v1":         true,
	"delivery.message.complained.v1":      true,
	"delivery.message.retry_scheduled.v1": true,
	"tracking.email_opened.v1":            true,
	"tracking.link_clicked.v1":            true,
	"tracking.recipient_unsubscribed.v1":  true,
	"suppression.recipient_suppressed.v1": true,
}

type WebhookConfig struct {
	ID              string
	WorkspaceID     string
	Name            string
	TargetURL       string
	Status          ConfigStatus
	Subscriptions   []string
	SecretHash      string
	SecretHint      string
	Version         int64
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DisabledAt      *time.Time
}

func NewWebhookConfig(id, workspaceID, name, targetURL string, subscriptions []string, createdByUserID string, now time.Time) (*WebhookConfig, error) {
	if id == "" {
		return nil, errors.New("webhook config id is required")
	}
	if workspaceID == "" {
		return nil, errors.New("workspace id is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("webhook name is required")
	}
	if _, err := url.ParseRequestURI(targetURL); err != nil {
		return nil, errors.New("valid target URL is required")
	}
	if len(subscriptions) == 0 {
		return nil, errors.New("at least one subscription is required")
	}
	return &WebhookConfig{
		ID:              id,
		WorkspaceID:     workspaceID,
		Name:            strings.TrimSpace(name),
		TargetURL:       targetURL,
		Status:          ConfigStatusActive,
		Subscriptions:   subscriptions,
		Version:         1,
		CreatedByUserID: createdByUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

type WebhookDelivery struct {
	ID               string
	WorkspaceID      string
	WebhookID        string
	SourceEventID    string
	SourceEventType  string
	Status           DeliveryStatus
	TargetURL        string
	AttemptCount     int64
	NextAttemptAt    *time.Time
	LastAttemptAt    *time.Time
	LastStatusCode   *int
	LastError        string
	RequestHeaders   map[string]string
	ResponseHeaders  map[string]string
	EventPayloadJSON map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Attempts         []WebhookDeliveryAttempt
}

type WebhookDeliveryAttempt struct {
	ID              string
	DeliveryID      string
	AttemptNumber   int64
	Status          string
	StatusCode      *int
	Error           string
	DurationMs      int64
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
	AttemptedAt     time.Time
}

type DeliveryFilter struct {
	WebhookID string
	Status    string
	EventType string
	From      *time.Time
	To        *time.Time
	Limit     int
	Cursor    string
}

type DeliveryResult struct {
	StatusCode      int
	DurationMs      int64
	Error           string
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
}

func ValidateWebhookConfig(cfg WebhookConfig) error {
	if cfg.WorkspaceID == "" {
		return ErrConfigInvalid
	}
	if cfg.Name == "" || len(cfg.Name) > 255 {
		return ErrConfigInvalid
	}
	if err := ValidateTargetURL(cfg.TargetURL); err != nil {
		return err
	}
	if err := ValidateSubscriptions(cfg.Subscriptions); err != nil {
		return err
	}
	return nil
}

var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

func ValidateTargetURL(rawURL string) error {
	if rawURL == "" {
		return ErrTargetURLInvalid
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ErrTargetURLInvalid
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return ErrTargetURLInvalid
	}
	if parsed.User != nil {
		return ErrTargetURLInvalid
	}
	if parsed.Host == "" {
		return ErrTargetURLInvalid
	}
	hostname := strings.ToLower(parsed.Hostname())
	if isPrivateHost(hostname) {
		return ErrTargetURLInvalid
	}
	if net.ParseIP(hostname) == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ips, err := lookupIPAddr(ctx, hostname)
		if err == nil {
			for _, ip := range ips {
				if IsPrivateIP(ip.IP) {
					return ErrTargetURLInvalid
				}
			}
		}
	}
	return nil
}

func ValidateSubscriptions(subscriptions []string) error {
	if len(subscriptions) == 0 {
		return ErrSubscriptionInvalid
	}
	for _, s := range subscriptions {
		if !AllowedSubscriptionEvents[s] {
			return ErrSubscriptionInvalid
		}
	}
	return nil
}

func CanDeliveryBeRetried(status DeliveryStatus) bool {
	return status == DeliveryStatusFailed || status == DeliveryStatusRetryScheduled
}

func IsTerminalDeliveryStatus(status DeliveryStatus) bool {
	return status == DeliveryStatusSucceeded || status == DeliveryStatusFailed
}

func IsRetryableHTTPStatus(statusCode int) bool {
	if statusCode == 0 {
		return true
	}
	if statusCode == 429 {
		return true
	}
	if statusCode >= 500 && statusCode < 600 {
		return true
	}
	return false
}

func isPrivateHost(hostname string) bool {
	if hostname == "localhost" || hostname == "::1" {
		return true
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return IsPrivateIP(ip)
	}
	return false
}

func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	return false
}

func SanitizeError(err string) string {
	if len(err) > 500 {
		return err[:500]
	}
	return err
}

func SanitizeHeaders(headers map[string]string) map[string]string {
	wrapped := make(map[string][]string, len(headers))
	for k, v := range headers {
		wrapped[k] = []string{v}
	}
	return httpheaders.SanitizeHeaders(wrapped)
}
