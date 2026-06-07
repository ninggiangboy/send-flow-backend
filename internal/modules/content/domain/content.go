package domain

import "time"

type TemplateStatus string

const (
	TemplateStatusDraft    TemplateStatus = "draft"
	TemplateStatusActive   TemplateStatus = "active"
	TemplateStatusArchived TemplateStatus = "archived"
)

type Template struct {
	ID               string
	WorkspaceID      string
	Name             string
	Status           TemplateStatus
	Subject          string
	SourceHTML       string
	SourceText       string
	Metadata         map[string]any
	CurrentVersionID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ArchivedAt       *time.Time
}

type TemplateVersion struct {
	ID            string
	WorkspaceID   string
	TemplateID    string
	VersionNumber int
	Subject       string
	SourceHTML    string
	SourceText    string
	Metadata      map[string]any
	PublishedAt   time.Time
	CreatedAt     time.Time
}

type RenderedTemplateSnapshot struct {
	ID                string
	WorkspaceID       string
	TemplateID        string
	TemplateVersionID string
	RenderInputHash   string
	Subject           string
	RenderedHTML      string
	RenderedText      string
	Warnings          []string
	CreatedAt         time.Time
}

type RenderResult struct {
	Subject  string
	HTML     string
	Text     string
	Warnings []string
}

func ValidTemplateStatus(s string) bool {
	switch TemplateStatus(s) {
	case TemplateStatusDraft, TemplateStatusActive, TemplateStatusArchived:
		return true
	default:
		return false
	}
}
