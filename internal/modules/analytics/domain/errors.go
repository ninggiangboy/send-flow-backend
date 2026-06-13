package domain

import "errors"

var (
	ErrAnalyticsEventInvalid       = errors.New("analytics event invalid")
	ErrAnalyticsEventDuplicate     = errors.New("analytics event duplicate")
	ErrAnalyticsProjectionNotFound = errors.New("analytics projection not found")
	ErrAnalyticsQueryInvalid       = errors.New("analytics query invalid")
	ErrAnalyticsStoreUnavailable   = errors.New("analytics store unavailable")
	ErrAnalyticsReadDenied         = errors.New("analytics read denied")
)
