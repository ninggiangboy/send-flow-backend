package domain

import (
	"net/url"
	"strings"
	"time"
)

const (
	EventTypeOpen        = "open"
	EventTypeClick       = "click"
	EventTypeUnsubscribe = "unsubscribe"

	SourceHTTP          = "http"
	SourceProviderEvent = "provider_event"

	LinkTypeClick = "click"
	LinkTypeOpen  = "open"
)

var validEventTypes = map[string]bool{
	EventTypeOpen:        true,
	EventTypeClick:       true,
	EventTypeUnsubscribe: true,
}

var validSources = map[string]bool{
	SourceHTTP:          true,
	SourceProviderEvent: true,
}

var validLinkTypes = map[string]bool{
	LinkTypeClick: true,
	LinkTypeOpen:  true,
}

type TrackingLink struct {
	ID             string
	WorkspaceID    string
	MessageID      string
	DestinationURL string
	LinkType       string
	Metadata       map[string]any
	CreatedAt      time.Time
	ExpiresAt      *time.Time
}

type TrackingEvent struct {
	ID                string
	WorkspaceID       string
	MessageID         string
	TrackingLinkID    string
	EventType         string
	Source            string
	SourceEventID     string
	Provider          string
	ProviderEventID   string
	ProviderMessageID string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	Metadata          map[string]any
	CreatedAt         time.Time
}

func ValidEventType(eventType string) bool {
	return validEventTypes[eventType]
}

func ValidSource(source string) bool {
	return validSources[source]
}

func ValidLinkType(linkType string) bool {
	return validLinkTypes[linkType]
}

func ValidateDestinationURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ErrDestinationInvalid
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ErrDestinationInvalid
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrDestinationInvalid
	}
	return nil
}
