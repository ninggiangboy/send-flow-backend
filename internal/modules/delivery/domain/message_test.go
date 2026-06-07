package domain

import (
	"testing"
)

func TestValidMessageStatus(t *testing.T) {
	valid := []string{"queued", "processing", "accepted", "delivered", "delayed", "bounced", "complained", "failed", "cancelled", "dlq"}
	for _, s := range valid {
		if !ValidMessageStatus(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if ValidMessageStatus("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidMessageSourceType(t *testing.T) {
	if !ValidMessageSourceType("campaign") {
		t.Error("expected campaign to be valid")
	}
	if !ValidMessageSourceType("transactional") {
		t.Error("expected transactional to be valid")
	}
	if ValidMessageSourceType("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidMessageType(t *testing.T) {
	if !ValidMessageType("marketing") {
		t.Error("expected marketing to be valid")
	}
	if !ValidMessageType("transactional") {
		t.Error("expected transactional to be valid")
	}
	if ValidMessageType("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidAttemptStatus(t *testing.T) {
	valid := []string{"started", "accepted", "temporary_failed", "permanent_failed"}
	for _, s := range valid {
		if !ValidAttemptStatus(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if ValidAttemptStatus("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidRetryStatus(t *testing.T) {
	valid := []string{"pending", "scheduled", "exhausted", "cancelled"}
	for _, s := range valid {
		if !ValidRetryStatus(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if ValidRetryStatus("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidateRecipientSnapshot(t *testing.T) {
	rs := RecipientSnapshot{EmailNormalized: "user@example.com"}
	if err := ValidateRecipientSnapshot(rs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	rs = RecipientSnapshot{EmailNormalized: ""}
	if err := ValidateRecipientSnapshot(rs); err == nil {
		t.Error("expected error for empty email_normalized")
	}
}

func TestValidateQueuedMessage(t *testing.T) {
	msg := Message{
		WorkspaceID:              "ws_1",
		RecipientEmailNormalized: "user@example.com",
		RecipientSnapshot:        RecipientSnapshot{EmailNormalized: "user@example.com"},
		TemplateID:               "tpl_1",
		SenderDomainID:           "sd_1",
		MessageType:              "marketing",
		SourceType:               "campaign",
	}
	if err := ValidateQueuedMessage(msg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	msg.WorkspaceID = ""
	if err := ValidateQueuedMessage(msg); err == nil {
		t.Error("expected error for empty workspace_id")
	}
}

func TestCanStartProcessing(t *testing.T) {
	m := Message{Status: MessageStatusQueued}
	if !m.CanStartProcessing() {
		t.Error("queued should be startable")
	}
	m.Status = MessageStatusProcessing
	if m.CanStartProcessing() {
		t.Error("processing should not be startable")
	}
	m.Status = MessageStatusDelivered
	if m.CanStartProcessing() {
		t.Error("delivered should not be startable")
	}
}

func TestCanMarkAccepted(t *testing.T) {
	m := Message{Status: MessageStatusProcessing}
	if !m.CanMarkAccepted() {
		t.Error("processing should be acceptable")
	}
	m.Status = MessageStatusQueued
	if m.CanMarkAccepted() {
		t.Error("queued should not be acceptable")
	}
	m.Status = MessageStatusDelivered
	if m.CanMarkAccepted() {
		t.Error("delivered should not be acceptable")
	}
}

func TestCanMarkDelivered(t *testing.T) {
	for _, status := range []string{MessageStatusAccepted, MessageStatusDelayed} {
		m := Message{Status: status}
		if !m.CanMarkDelivered() {
			t.Errorf("%s should be deliverable", status)
		}
	}
	m := Message{Status: MessageStatusQueued}
	if m.CanMarkDelivered() {
		t.Error("queued should not be deliverable")
	}
}

func TestCanMarkBounced(t *testing.T) {
	canBounce := []string{MessageStatusQueued, MessageStatusProcessing, MessageStatusAccepted, MessageStatusDelayed}
	for _, s := range canBounce {
		m := Message{Status: s}
		if !m.CanMarkBounced() {
			t.Errorf("expected %q to be bouncable", s)
		}
	}
	terminal := []string{MessageStatusDelivered, MessageStatusBounced, MessageStatusComplained, MessageStatusFailed, MessageStatusCancelled, MessageStatusDLQ}
	for _, s := range terminal {
		m := Message{Status: s}
		if m.CanMarkBounced() {
			t.Errorf("expected %q to not be bouncable", s)
		}
	}
}

func TestCanMarkComplained(t *testing.T) {
	canComplain := []string{MessageStatusAccepted, MessageStatusDelivered, MessageStatusDelayed}
	for _, s := range canComplain {
		m := Message{Status: s}
		if !m.CanMarkComplained() {
			t.Errorf("expected %q to be complainable", s)
		}
	}
	m := Message{Status: MessageStatusQueued}
	if m.CanMarkComplained() {
		t.Error("queued should not be complainable")
	}
}

func TestCanFail(t *testing.T) {
	nonTerminal := []string{MessageStatusQueued, MessageStatusProcessing, MessageStatusAccepted, MessageStatusDelayed}
	for _, s := range nonTerminal {
		m := Message{Status: s}
		if !m.CanFail() {
			t.Errorf("expected %q to be fail-able", s)
		}
	}
	terminal := []string{MessageStatusDelivered, MessageStatusBounced, MessageStatusComplained, MessageStatusFailed, MessageStatusCancelled, MessageStatusDLQ}
	for _, s := range terminal {
		m := Message{Status: s}
		if m.CanFail() {
			t.Errorf("expected %q to not be fail-able", s)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []string{MessageStatusDelivered, MessageStatusBounced, MessageStatusComplained, MessageStatusFailed, MessageStatusCancelled, MessageStatusDLQ}
	for _, s := range terminal {
		m := Message{Status: s}
		if !m.IsTerminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}
	nonTerminal := []string{MessageStatusQueued, MessageStatusProcessing, MessageStatusAccepted, MessageStatusDelayed}
	for _, s := range nonTerminal {
		m := Message{Status: s}
		if m.IsTerminal() {
			t.Errorf("expected %q to not be terminal", s)
		}
	}
}
