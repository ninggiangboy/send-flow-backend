package app

import (
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
)

// --- Contact DTOs ---

type ContactDTO struct {
	ID              string
	WorkspaceID     string
	Email           string
	EmailNormalized string
	FirstName       string
	LastName        string
	Status          string
	Tags            []string
	Attributes      map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

func contactToDTO(c domain.Contact) ContactDTO {
	return ContactDTO{
		ID:              c.ID,
		WorkspaceID:     c.WorkspaceID,
		Email:           c.Email,
		EmailNormalized: c.EmailNormalized,
		FirstName:       c.FirstName,
		LastName:        c.LastName,
		Status:          string(c.Status),
		Tags:            c.Tags,
		Attributes:      c.Attributes,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
		ArchivedAt:      c.ArchivedAt,
	}
}

func contactSliceToDTOs(contacts []domain.Contact) []ContactDTO {
	result := make([]ContactDTO, len(contacts))
	for i, c := range contacts {
		result[i] = contactToDTO(c)
	}
	return result
}

type ContactResult struct {
	Contact ContactDTO
}

type ContactListResult struct {
	Contacts   []ContactDTO
	NextCursor string
}

// --- List DTOs ---

type AudienceListDTO struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ArchivedAt  *time.Time
}

func listToDTO(l domain.AudienceList) AudienceListDTO {
	return AudienceListDTO{
		ID:          l.ID,
		WorkspaceID: l.WorkspaceID,
		Name:        l.Name,
		Description: l.Description,
		Metadata:    l.Metadata,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
		ArchivedAt:  l.ArchivedAt,
	}
}

func listSliceToDTOs(lists []domain.AudienceList) []AudienceListDTO {
	result := make([]AudienceListDTO, len(lists))
	for i, l := range lists {
		result[i] = listToDTO(l)
	}
	return result
}

type ListResult struct {
	List AudienceListDTO
}

type ListListResult struct {
	Lists      []AudienceListDTO
	Counts     map[string]int64
	NextCursor string
}

type MembershipUpdateResultDTO struct {
	AddedCount   int
	RemovedCount int
	SkippedCount int
}

type MembershipUpdateResult struct {
	Result MembershipUpdateResultDTO
}

// --- Segment DTOs ---

type SegmentDTO struct {
	ID             string
	WorkspaceID    string
	Name           string
	DefinitionJSON map[string]any
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func segmentToDTO(s domain.Segment) SegmentDTO {
	return SegmentDTO{
		ID:             s.ID,
		WorkspaceID:    s.WorkspaceID,
		Name:           s.Name,
		DefinitionJSON: s.DefinitionJSON,
		Status:         string(s.Status),
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

func segmentSliceToDTOs(segments []domain.Segment) []SegmentDTO {
	result := make([]SegmentDTO, len(segments))
	for i, s := range segments {
		result[i] = segmentToDTO(s)
	}
	return result
}

type SegmentResult struct {
	Segment SegmentDTO
}

type SegmentListResult struct {
	Segments   []SegmentDTO
	NextCursor string
}

// --- Import Job DTOs ---

type ImportJobDTO struct {
	ID             string
	WorkspaceID    string
	SourceURI      string
	DedupeMode     string
	Status         string
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

func importJobToDTO(j domain.AudienceImportJob) ImportJobDTO {
	return ImportJobDTO{
		ID:             j.ID,
		WorkspaceID:    j.WorkspaceID,
		SourceURI:      j.SourceURI,
		DedupeMode:     string(j.DedupeMode),
		Status:         string(j.Status),
		ProcessedCount: j.ProcessedCount,
		CreatedCount:   j.CreatedCount,
		UpdatedCount:   j.UpdatedCount,
		FailedCount:    j.FailedCount,
		ErrorSummary:   j.ErrorSummary,
		Metadata:       j.Metadata,
		CreatedAt:      j.CreatedAt,
		UpdatedAt:      j.UpdatedAt,
		CompletedAt:    j.CompletedAt,
	}
}

func importJobSliceToDTOs(jobs []domain.AudienceImportJob) []ImportJobDTO {
	result := make([]ImportJobDTO, len(jobs))
	for i, j := range jobs {
		result[i] = importJobToDTO(j)
	}
	return result
}

type ImportJobResult struct {
	Job ImportJobDTO
}

type ImportJobListResult struct {
	Jobs       []ImportJobDTO
	NextCursor string
}

// --- Export Job DTOs ---

type ExportJobDTO struct {
	ID                  string
	WorkspaceID         string
	FiltersJSON         map[string]any
	SelectedFields      []string
	Format              string
	ZipOutput           bool
	Status              string
	ProcessedCount      int64
	EstimatedTotalCount int64
	ArtifactURI         string
	DownloadURL         string
	DownloadURLTTL      *time.Time
	ErrorSummary        string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	CompletedAt         *time.Time
}

func exportJobToDTO(j domain.AudienceExportJob) ExportJobDTO {
	return ExportJobDTO{
		ID:                  j.ID,
		WorkspaceID:         j.WorkspaceID,
		FiltersJSON:         j.FiltersJSON,
		SelectedFields:      j.SelectedFields,
		Format:              string(j.Format),
		ZipOutput:           j.ZipOutput,
		Status:              string(j.Status),
		ProcessedCount:      j.ProcessedCount,
		EstimatedTotalCount: j.EstimatedTotalCount,
		ArtifactURI:         j.ArtifactURI,
		ErrorSummary:        j.ErrorSummary,
		CreatedAt:           j.CreatedAt,
		UpdatedAt:           j.UpdatedAt,
		CompletedAt:         j.CompletedAt,
	}
}

type ExportJobResult struct {
	Job ExportJobDTO
}

type ExportJobListResult struct {
	Jobs       []ExportJobDTO
	NextCursor string
}

// --- Audience Selection types ---

type AudienceSelectionRef struct {
	ListID     string
	SegmentID  string
	ContactIDs []string
}

type AudienceRecipient struct {
	ContactID       string
	Email           string
	EmailNormalized string
	FirstName       string
	LastName        string
	Tags            []string
	Attributes      map[string]any
}

// --- Input types used by API callers ---

type CreateContactInput struct {
	WorkspaceID string
	UserID      string
	Email       string
	FirstName   string
	LastName    string
	Tags        []string
	Attributes  map[string]any
	Now         time.Time
}

type UpdateContactInput struct {
	WorkspaceID string
	ContactID   string
	UserID      string
	Email       *string
	FirstName   *string
	LastName    *string
	Status      *string
	Tags        []string
	Attributes  map[string]any
	Now         time.Time
}
