package domain

import "time"

type ContactStatus string

const (
	ContactStatusActive   ContactStatus = "active"
	ContactStatusArchived ContactStatus = "archived"
)

type SegmentStatus string

const (
	SegmentStatusReady      SegmentStatus = "ready"
	SegmentStatusProcessing SegmentStatus = "processing"
	SegmentStatusDisabled   SegmentStatus = "disabled"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type DedupeMode string

const (
	DedupeModeByEmail DedupeMode = "by_email"
)

type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatJSON ExportFormat = "json"
)

type ListMembershipMode string

const (
	ListMembershipModeMerge   ListMembershipMode = "merge"
	ListMembershipModeReplace ListMembershipMode = "replace"
)

type Contact struct {
	ID              string
	WorkspaceID     string
	Email           string
	EmailNormalized string
	FirstName       string
	LastName        string
	Status          ContactStatus
	Tags            []string
	Attributes      map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

type AudienceList struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ArchivedAt  *time.Time
}

type AudienceListMembership struct {
	WorkspaceID string
	ListID      string
	ContactID   string
	CreatedAt   time.Time
}

type Segment struct {
	ID             string
	WorkspaceID    string
	Name           string
	DefinitionJSON map[string]any
	Status         SegmentStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AudienceImportJob struct {
	ID             string
	WorkspaceID    string
	SourceURI      string
	DedupeMode     DedupeMode
	Status         JobStatus
	ProcessedCount int64
	CreatedCount   int64
	UpdatedCount   int64
	FailedCount    int64
	ErrorSummary   string
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

type AudienceExportJob struct {
	ID             string
	WorkspaceID    string
	FiltersJSON    map[string]any
	SelectedFields []string
	Format         ExportFormat
	Status         JobStatus
	ArtifactURI    string
	ErrorSummary   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

type MembershipUpdateResult struct {
	AddedCount   int
	RemovedCount int
	SkippedCount int
}
