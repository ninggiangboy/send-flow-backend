package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
)

var transparentPixel = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00,
	0x21, 0xf9, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
	0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b,
}

type trackingHTTP struct {
	svc *trackingapp.Service
}

func newTrackingHTTP(svc *trackingapp.Service) *trackingHTTP {
	return &trackingHTTP{svc: svc}
}

func (h *trackingHTTP) serveOpenPixel(w http.ResponseWriter, r *http.Request) {
	trackingID := chi.URLParam(r, "tracking_id")
	if trackingID != "" {
		h.svc.RecordOpen(r.Context(), trackingapp.RecordOpenInput{
			TrackingID: trackingID,
			Source:     "http",
		})
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusOK)
	w.Write(transparentPixel)
}

func (h *trackingHTTP) serveClickRedirect(w http.ResponseWriter, r *http.Request) {
	trackingID := chi.URLParam(r, "tracking_id")
	if trackingID == "" {
		writeError(w, r, http.StatusNotFound, "tracking.invalid_tracking_id", "tracking id not found", nil)
		return
	}

	result, err := h.svc.RecordClick(r.Context(), trackingapp.RecordClickInput{
		TrackingID: trackingID,
		Source:     "http",
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
		return
	}

	if result == nil || result.DestinationURL == "" {
		writeError(w, r, http.StatusNotFound, "tracking.invalid_tracking_id", "tracking id not found", nil)
		return
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	http.Redirect(w, r, result.DestinationURL, http.StatusFound)
}

func (h *trackingHTTP) serveUnsubscribe(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, r, http.StatusBadRequest, "suppression.unsubscribe_token_invalid", "invalid unsubscribe token", nil)
		return
	}

	_, err := h.svc.RecordUnsubscribe(r.Context(), trackingapp.RecordUnsubscribeInput{
		Token:  token,
		Source: "http",
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html><html><body><p>You have been unsubscribed.</p></body></html>`))
}
