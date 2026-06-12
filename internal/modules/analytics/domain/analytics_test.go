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

func TestValidateIntervalValid(t *testing.T) {
	for _, v := range []string{"hour", "day", "week"} {
		if err := ValidateInterval(v); err != nil {
			t.Fatalf("expected nil for %q, got %v", v, err)
		}
	}
}

func TestValidateIntervalInvalid(t *testing.T) {
	err := ValidateInterval("month")
	if err == nil {
		t.Fatal("expected error for invalid interval")
	}
}

func TestValidateGroupByValid(t *testing.T) {
	for _, v := range []string{"provider", "recipient_domain", "event_type"} {
		if err := ValidateGroupBy(v); err != nil {
			t.Fatalf("expected nil for %q, got %v", v, err)
		}
	}
}

func TestValidateGroupByInvalid(t *testing.T) {
	err := ValidateGroupBy("unknown")
	if err == nil {
		t.Fatal("expected error for invalid group")
	}
}

func TestValidateTimeRangeValid(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := ValidateTimeRange(from, to); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateTimeRangeZero(t *testing.T) {
	if err := ValidateTimeRange(time.Time{}, time.Time{}); err != nil {
		t.Fatalf("expected nil for zero times, got %v", err)
	}
}

func TestValidateTimeRangeFromAfterTo(t *testing.T) {
	from := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	err := ValidateTimeRange(from, to)
	if err == nil {
		t.Fatal("expected error for from > to")
	}
}

func TestCampaignQueryFilterValidateValid(t *testing.T) {
	f := CampaignQueryFilter{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		Limit:       50,
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCampaignQueryFilterValidateMissingWorkspaceID(t *testing.T) {
	f := CampaignQueryFilter{
		CampaignID: "camp-1",
		Limit:      50,
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestCampaignQueryFilterValidateMissingCampaignID(t *testing.T) {
	f := CampaignQueryFilter{
		WorkspaceID: "ws-1",
		Limit:       50,
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("expected error for missing campaign_id")
	}
}

func TestCampaignQueryFilterValidateInvalidEventType(t *testing.T) {
	f := CampaignQueryFilter{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		EventType:   "unknown",
		Limit:       50,
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("expected error for invalid event_type")
	}
}

func TestCampaignQueryFilterValidateInvalidLimit(t *testing.T) {
	f := CampaignQueryFilter{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		Limit:       200,
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("expected error for limit > 100")
	}
}

func TestCampaignQueryFilterValidateZeroLimit(t *testing.T) {
	f := CampaignQueryFilter{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		Limit:       0,
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("expected error for limit = 0")
	}
}
