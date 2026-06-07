package domain

import (
	"testing"
	"time"
)

func TestValidCampaignStatus(t *testing.T) {
	valid := []string{"draft", "scheduled", "running", "paused", "cancelled", "completed"}
	for _, s := range valid {
		if !ValidCampaignStatus(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if ValidCampaignStatus("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidAudienceType(t *testing.T) {
	if !ValidAudienceType("contacts") {
		t.Error("expected contacts to be valid")
	}
	if !ValidAudienceType("list") {
		t.Error("expected list to be valid")
	}
	if !ValidAudienceType("segment") {
		t.Error("expected segment to be valid")
	}
	if ValidAudienceType("invalid") {
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

func TestValidCandidateStatus(t *testing.T) {
	if !ValidCandidateStatus("planned") {
		t.Error("expected planned to be valid")
	}
	if !ValidCandidateStatus("queued") {
		t.Error("expected queued to be valid")
	}
	if !ValidCandidateStatus("skipped") {
		t.Error("expected skipped to be valid")
	}
	if ValidCandidateStatus("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidateCampaignName(t *testing.T) {
	if err := ValidateCampaignName("My Campaign"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateCampaignName(""); err == nil {
		t.Error("expected error for empty name")
	}
	if err := ValidateCampaignName("   "); err == nil {
		t.Error("expected error for whitespace-only name")
	}
}

func TestValidateAudienceRef(t *testing.T) {
	err := ValidateAudienceRef(AudienceRef{Type: AudienceTypeContacts, ContactIDs: []string{"ct_1"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = ValidateAudienceRef(AudienceRef{Type: AudienceTypeContacts, ContactIDs: []string{}})
	if err == nil {
		t.Error("expected error for empty contact_ids")
	}
	err = ValidateAudienceRef(AudienceRef{Type: AudienceTypeList, ID: "list_1"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = ValidateAudienceRef(AudienceRef{Type: AudienceTypeList})
	if err == nil {
		t.Error("expected error for list without id")
	}
	err = ValidateAudienceRef(AudienceRef{Type: AudienceTypeSegment, ID: "seg_1"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = ValidateAudienceRef(AudienceRef{Type: AudienceTypeSegment})
	if err == nil {
		t.Error("expected error for segment without id")
	}
}

func TestCampaignCanUpdateDraft(t *testing.T) {
	c := Campaign{Status: CampaignStatusDraft}
	if !c.CanUpdateDraft() {
		t.Error("draft should be updatable")
	}
	c.Status = CampaignStatusScheduled
	if c.CanUpdateDraft() {
		t.Error("scheduled should not be updatable")
	}
}

func TestCampaignCanSchedule(t *testing.T) {
	c := Campaign{Status: CampaignStatusDraft}
	if !c.CanSchedule() {
		t.Error("draft should be schedulable")
	}
	c.Status = CampaignStatusScheduled
	if c.CanSchedule() {
		t.Error("scheduled should not be schedulable")
	}
}

func TestCampaignCanPause(t *testing.T) {
	c := Campaign{Status: CampaignStatusScheduled}
	if !c.CanPause() {
		t.Error("scheduled should be pausable")
	}
	c.Status = CampaignStatusRunning
	if !c.CanPause() {
		t.Error("running should be pausable")
	}
	c.Status = CampaignStatusDraft
	if c.CanPause() {
		t.Error("draft should not be pausable")
	}
	c.Status = CampaignStatusPaused
	if c.CanPause() {
		t.Error("already paused should not be pausable")
	}
}

func TestCampaignCanResume(t *testing.T) {
	c := Campaign{Status: CampaignStatusPaused}
	if !c.CanResume() {
		t.Error("paused should be resumable")
	}
	c.Status = CampaignStatusDraft
	if c.CanResume() {
		t.Error("draft should not be resumable")
	}
}

func TestCampaignCanCancel(t *testing.T) {
	for _, status := range []CampaignStatus{CampaignStatusDraft, CampaignStatusScheduled, CampaignStatusPaused} {
		c := Campaign{Status: status}
		if !c.CanCancel() {
			t.Errorf("%s should be cancellable", status)
		}
	}
	c := Campaign{Status: CampaignStatusRunning}
	if c.CanCancel() {
		t.Error("running should not be cancellable")
	}
	c = Campaign{Status: CampaignStatusCompleted}
	if c.CanCancel() {
		t.Error("completed should not be cancellable")
	}
}

func TestCampaignDefaults(t *testing.T) {
	now := time.Now()
	c := Campaign{
		ID:          "cmp_1",
		WorkspaceID: "ws_1",
		Name:        "Test Campaign",
		Status:      CampaignStatusDraft,
		MessageType: MessageTypeMarketing,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if c.ID != "cmp_1" {
		t.Errorf("expected cmp_1, got %s", c.ID)
	}
	if string(c.Status) != "draft" {
		t.Errorf("expected draft, got %s", c.Status)
	}
	if string(c.MessageType) != "marketing" {
		t.Errorf("expected marketing, got %s", c.MessageType)
	}
}
