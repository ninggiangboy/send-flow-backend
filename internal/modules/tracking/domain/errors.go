package domain

import "errors"

var (
	ErrTrackingLinkNotFound    = errors.New("tracking link not found")
	ErrTrackingLinkExpired     = errors.New("tracking link expired")
	ErrTrackingEventInvalid    = errors.New("tracking event invalid")
	ErrTrackingEventConflict   = errors.New("tracking event conflict")
	ErrDestinationInvalid      = errors.New("destination URL invalid")
	ErrUnsubscribeTokenInvalid = errors.New("unsubscribe token invalid")
)
