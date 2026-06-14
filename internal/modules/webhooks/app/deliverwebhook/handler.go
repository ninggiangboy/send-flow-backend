package deliverwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type Options struct {
	Deliverer ports.HTTPDeliverer
	IDGen     func() (string, error)
	Clock     func() time.Time
	Logger    *slog.Logger
}

type Command struct {
	WorkspaceID     string
	WebhookID       string
	DeliveryID      string
	TargetURL       string
	SecretHash      string
	EventPayload    map[string]any
	SourceEventID   string
	SourceEventType string
	AttemptNumber   int64
}

type Result struct {
	Success        bool
	StatusCode     int
	Error          string
	DurationMs     int64
	Attempt        domain.WebhookDeliveryAttempt
	DeliveryResult domain.DeliveryResult
}

type Handler struct {
	deliverer ports.HTTPDeliverer
	idGen     func() (string, error)
	clock     func() time.Time
	log       *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		deliverer: opts.Deliverer,
		idGen:     opts.IDGen,
		clock:     opts.Clock,
		log:       opts.Logger.With("usecase", "deliver_webhook"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
	now := h.clock()
	timestamp := fmt.Sprintf("%d", now.Unix())

	payloadJSON, err := json.Marshal(cmd.EventPayload)
	if err != nil {
		return nil, err
	}

	signature := SignPayloadRaw(payloadJSON, timestamp, cmd.SecretHash)

	deliveryReq := ports.DeliveryHTTPRequest{
		URL:             cmd.TargetURL,
		Body:            payloadJSON,
		SignatureHeader: "Sendflow-Signature",
		SignatureValue:  signature,
		TimestampHeader: "Sendflow-Timestamp",
		TimestampValue:  timestamp,
		EventIDHeader:   "Sendflow-Event-ID",
		EventIDValue:    cmd.SourceEventID,
	}

	resp, deliverErr := h.deliverer.Deliver(ctx, deliveryReq)

	attemptID, err := h.idGen()
	if err != nil {
		return nil, err
	}

	isSuccess := deliverErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 300

	var attempt domain.WebhookDeliveryAttempt
	var deliveryResult domain.DeliveryResult

	if deliverErr != nil {
		attempt = domain.WebhookDeliveryAttempt{
			ID:             attemptID,
			DeliveryID:     cmd.DeliveryID,
			AttemptNumber:  cmd.AttemptNumber,
			Status:         string(domain.DeliveryStatusFailed),
			Error:          domain.SanitizeError(deliverErr.Error()),
			RequestHeaders: map[string]string{},
			AttemptedAt:    now,
		}
		deliveryResult = domain.DeliveryResult{
			Error:          domain.SanitizeError(deliverErr.Error()),
			RequestHeaders: map[string]string{},
		}
	} else {
		status := string(domain.DeliveryStatusFailed)
		if isSuccess {
			status = string(domain.DeliveryStatusSucceeded)
		}
		attempt = domain.WebhookDeliveryAttempt{
			ID:              attemptID,
			DeliveryID:      cmd.DeliveryID,
			AttemptNumber:   cmd.AttemptNumber,
			Status:          status,
			StatusCode:      &resp.StatusCode,
			Error:           domain.SanitizeError(resp.Error),
			DurationMs:      resp.DurationMs,
			RequestHeaders:  map[string]string{},
			ResponseHeaders: domain.SanitizeHeaders(resp.Headers),
			AttemptedAt:     now,
		}
		if resp.StatusCode == 0 {
			attempt.StatusCode = nil
		}
		deliveryResult = domain.DeliveryResult{
			StatusCode:      resp.StatusCode,
			DurationMs:      resp.DurationMs,
			Error:           domain.SanitizeError(resp.Error),
			RequestHeaders:  map[string]string{},
			ResponseHeaders: domain.SanitizeHeaders(resp.Headers),
		}
	}

	return &Result{
		Success:        isSuccess,
		StatusCode:     resp.StatusCode,
		Error:          attempt.Error,
		DurationMs:     attempt.DurationMs,
		Attempt:        attempt,
		DeliveryResult: deliveryResult,
	}, nil
}

func SignPayloadRaw(payload []byte, timestamp, signingKey string) string {
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
