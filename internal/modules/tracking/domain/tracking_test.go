package domain

import (
	"testing"
	"time"
)

func TestValidEventType(t *testing.T) {
	tests := []struct {
		eventType string
		valid     bool
	}{
		{EventTypeOpen, true},
		{EventTypeClick, true},
		{EventTypeUnsubscribe, true},
		{"bounced", false},
		{"delivered", false},
		{"", false},
	}
	for _, tc := range tests {
		got := ValidEventType(tc.eventType)
		if got != tc.valid {
			t.Errorf("ValidEventType(%q) = %v, want %v", tc.eventType, got, tc.valid)
		}
	}
}

func TestValidSource(t *testing.T) {
	tests := []struct {
		source string
		valid  bool
	}{
		{SourceHTTP, true},
		{SourceProviderEvent, true},
		{"webhook", false},
		{"", false},
	}
	for _, tc := range tests {
		got := ValidSource(tc.source)
		if got != tc.valid {
			t.Errorf("ValidSource(%q) = %v, want %v", tc.source, got, tc.valid)
		}
	}
}

func TestValidateDestinationURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com", false},
		{"http://example.com/path?q=1", false},
		{"https://sub.example.com:8080/page", false},
		{"", true},
		{"  ", true},
		{"javascript:alert(1)", true},
		{"ftp://example.com", true},
		{"file:///etc/passwd", true},
		{"data:text/html,<script>alert(1)</script>", true},
	}
	for _, tc := range tests {
		err := ValidateDestinationURL(tc.url)
		gotErr := err != nil
		if gotErr != tc.wantErr {
			t.Errorf("ValidateDestinationURL(%q) error = %v, wantErr = %v", tc.url, err, tc.wantErr)
		}
	}
}

func TestTrackingLinkValidation(t *testing.T) {
	link := TrackingLink{
		ID:             "tl_1",
		WorkspaceID:    "ws_1",
		MessageID:      "msg_1",
		DestinationURL: "https://example.com",
		LinkType:       LinkTypeClick,
		CreatedAt:      time.Now().UTC(),
	}
	if !ValidLinkType(link.LinkType) {
		t.Errorf("expected valid link type")
	}
	if err := ValidateDestinationURL(link.DestinationURL); err != nil {
		t.Errorf("expected valid destination URL, got %v", err)
	}
}

func TestTrackingEventValidation(t *testing.T) {
	evt := TrackingEvent{
		ID:          "te_1",
		WorkspaceID: "ws_1",
		MessageID:   "msg_1",
		EventType:   EventTypeOpen,
		Source:      SourceHTTP,
		OccurredAt:  time.Now().UTC(),
		ReceivedAt:  time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
	}
	if !ValidEventType(evt.EventType) {
		t.Errorf("expected valid event type")
	}
	if !ValidSource(evt.Source) {
		t.Errorf("expected valid source")
	}
}
