package domain

import (
	"testing"
)

func TestValidTemplateStatus(t *testing.T) {
	if !ValidTemplateStatus("draft") {
		t.Error("expected draft to be valid")
	}
	if !ValidTemplateStatus("active") {
		t.Error("expected active to be valid")
	}
	if !ValidTemplateStatus("archived") {
		t.Error("expected archived to be valid")
	}
	if ValidTemplateStatus("invalid") {
		t.Error("expected invalid status to be invalid")
	}
}

func TestTemplateDefaults(t *testing.T) {
	tmpl := Template{
		ID:         "tpl_1",
		Name:       "Test Template",
		Status:     TemplateStatusDraft,
		Subject:    "Hello {{name}}",
		SourceHTML: "<p>Hello {{name}}</p>",
	}

	if tmpl.ID != "tpl_1" {
		t.Errorf("expected ID tpl_1, got %s", tmpl.ID)
	}
	if string(tmpl.Status) != "draft" {
		t.Errorf("expected status draft, got %s", tmpl.Status)
	}
	if tmpl.Name != "Test Template" {
		t.Errorf("expected name 'Test Template', got %s", tmpl.Name)
	}
}

func TestTemplateVersion(t *testing.T) {
	v := TemplateVersion{
		ID:            "ver_1",
		TemplateID:    "tpl_1",
		VersionNumber: 1,
		Subject:       "Hello {{name}}",
		SourceHTML:    "<p>Hello {{name}}</p>",
	}

	if v.VersionNumber != 1 {
		t.Errorf("expected version number 1, got %d", v.VersionNumber)
	}
	if v.Subject != "Hello {{name}}" {
		t.Errorf("expected subject 'Hello {{name}}', got %s", v.Subject)
	}
}

func TestRenderResult(t *testing.T) {
	r := RenderResult{
		Subject:  "Hello Alice",
		HTML:     "<p>Hello Alice</p>",
		Text:     "Hello Alice",
		Warnings: []string{},
	}

	if r.Subject != "Hello Alice" {
		t.Errorf("expected subject 'Hello Alice', got %s", r.Subject)
	}
	if r.HTML != "<p>Hello Alice</p>" {
		t.Errorf("expected HTML '<p>Hello Alice</p>', got %s", r.HTML)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(r.Warnings))
	}
}
