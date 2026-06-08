package api

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	ingestionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
)

const maxWebhookBodySize = 256 * 1024

type ingestionHTTP struct {
	svc *ingestionapp.Service
}

func newIngestionHTTP(svc *ingestionapp.Service) *ingestionHTTP {
	return &ingestionHTTP{svc: svc}
}

func (h *ingestionHTTP) ingestProviderWebhook(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodySize)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "webhook.payload_invalid", "request body too large or unreadable", nil)
		return
	}

	if len(rawBody) == 0 {
		writeError(w, r, http.StatusBadRequest, "webhook.payload_invalid", "empty request body", nil)
		return
	}

	headers := make(map[string][]string, len(r.Header))
	for k, v := range r.Header {
		headers[k] = v
	}

	result, err := h.svc.IngestProviderWebhook(r.Context(), ingestionapp.IngestProviderWebhookInput{
		Provider:   provider,
		Headers:    headers,
		RawBody:    rawBody,
		ReceivedAt: time.Now().UTC(),
	})
	if err != nil {
		writeIngestionErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"accepted": result.Accepted,
	})
}

func writeIngestionErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrProviderNotSupported):
		writeError(w, r, http.StatusNotFound, "webhook.provider_not_supported", "provider not supported", nil)
	case errors.Is(err, domain.ErrInvalidSignature):
		writeError(w, r, http.StatusUnauthorized, "webhook.invalid_signature", "invalid signature", nil)
	case errors.Is(err, domain.ErrPayloadInvalid):
		writeError(w, r, http.StatusBadRequest, "webhook.payload_invalid", "payload invalid", nil)
	case errors.Is(err, domain.ErrDuplicateEventConflict):
		writeError(w, r, http.StatusConflict, "webhook.duplicate_event_conflict", "duplicate event conflict", nil)
	case errors.Is(err, domain.ErrTemporarilyUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "webhook.ingest_temporarily_unavailable", "service temporarily unavailable", nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}
