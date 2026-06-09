package domain

import (
	"testing"
	"time"
)

func TestValidateFactValid(t *testing.T) {
	err := ValidateFact("ws_1", "event_1", EventTypeDelivered, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateFactMissingWorkspaceID(t *testing.T) {
	err := ValidateFact("", "event_1", EventTypeDelivered, time.Now(), time.Now())
	if err != ErrAnalyticsEventInvalid {
		t.Fatalf("expected ErrAnalyticsEventInvalid, got %v", err)
	}
}

func TestValidateFactMissingSourceEventID(t *testing.T) {
	err := ValidateFact("ws_1", "", EventTypeDelivered, time.Now(), time.Now())
	if err != ErrAnalyticsEventInvalid {
		t.Fatalf("expected ErrAnalyticsEventInvalid, got %v", err)
	}
}

func TestValidateFactUnknownEventType(t *testing.T) {
	err := ValidateFact("ws_1", "event_1", "unknown_type", time.Now(), time.Now())
	if err != ErrAnalyticsEventInvalid {
		t.Fatalf("expected ErrAnalyticsEventInvalid, got %v", err)
	}
}

func TestValidateFactZeroOccurredAt(t *testing.T) {
	err := ValidateFact("ws_1", "event_1", EventTypeDelivered, time.Time{}, time.Now())
	if err != ErrAnalyticsEventInvalid {
		t.Fatalf("expected ErrAnalyticsEventInvalid, got %v", err)
	}
}

func TestValidateFactZeroReceivedAt(t *testing.T) {
	err := ValidateFact("ws_1", "event_1", EventTypeDelivered, time.Now(), time.Time{})
	if err != ErrAnalyticsEventInvalid {
		t.Fatalf("expected ErrAnalyticsEventInvalid, got %v", err)
	}
}

func TestNormalizeStringLowercases(t *testing.T) {
	result := NormalizeString("  EXAMPLE.COM  ")
	if result != "example.com" {
		t.Fatalf("expected example.com, got %s", result)
	}
}

func TestNormalizeStringEmpty(t *testing.T) {
	result := NormalizeString("")
	if result != "" {
		t.Fatalf("expected empty, got %s", result)
	}
}

func TestKnownEventTypes(t *testing.T) {
	expected := []string{
		EventTypeQueued,
		EventTypeAccepted,
		EventTypeDelivered,
		EventTypeBounced,
		EventTypeComplained,
		EventTypeRetryScheduled,
		EventTypeOpened,
		EventTypeClicked,
		EventTypeUnsubscribed,
		EventTypeSuppressed,
	}
	for _, et := range expected {
		if !KnownEventTypes[et] {
			t.Fatalf("expected %s to be known", et)
		}
	}
}

func TestComputeRateZeroDenominator(t *testing.T) {
	rate := ComputeRate(5, 0)
	if rate != 0 {
		t.Fatalf("expected 0, got %f", rate)
	}
}

func TestComputeRateNormal(t *testing.T) {
	rate := ComputeRate(25, 100)
	if rate != 25.0 {
		t.Fatalf("expected 25.0, got %f", rate)
	}
}

func TestComputeRateZeroNumerator(t *testing.T) {
	rate := ComputeRate(0, 100)
	if rate != 0 {
		t.Fatalf("expected 0, got %f", rate)
	}
}
