package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type ContactListQuery struct {
	WorkspaceID string
	Q           string
	Status      string
	ListID      string
	SegmentID   string
	Limit       int
	Cursor      string
	Sort        string
}

type ListListQuery struct {
	WorkspaceID string
	Limit       int
	Cursor      string
}

type SegmentListQuery struct {
	WorkspaceID string
	Status      string
	Limit       int
	Cursor      string
}

type ImportJobListQuery struct {
	WorkspaceID string
	Status      string
	Limit       int
	Cursor      string
}

type ContactReadRepository interface {
	FindContactByID(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error)
	FindContactByEmail(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error)
	ListContacts(ctx context.Context, query ContactListQuery) ([]domain.Contact, string, error)
}

type ContactWriteRepository interface {
	CreateContact(ctx context.Context, contact domain.Contact) error
	UpdateContact(ctx context.Context, contact domain.Contact) error
	ArchiveContact(ctx context.Context, workspaceID, contactID string, archivedAt time.Time) error
}

type ListReadRepository interface {
	FindListByID(ctx context.Context, workspaceID, listID string) (*domain.AudienceList, error)
	ListLists(ctx context.Context, query ListListQuery) ([]domain.AudienceList, string, error)
	CountContactsByList(ctx context.Context, workspaceID string, listIDs []string) (map[string]int64, error)
}

type ListWriteRepository interface {
	CreateList(ctx context.Context, list domain.AudienceList) error
	ReplaceListMemberships(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error)
	MergeListMemberships(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error)
}

type SegmentReadRepository interface {
	FindSegmentByID(ctx context.Context, workspaceID, segmentID string) (*domain.Segment, error)
	ListSegments(ctx context.Context, query SegmentListQuery) ([]domain.Segment, string, error)
}

type SegmentWriteRepository interface {
	CreateSegment(ctx context.Context, segment domain.Segment) error
	UpdateSegment(ctx context.Context, segment domain.Segment) error
}

type ImportJobReadRepository interface {
	FindImportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceImportJob, error)
	ListImportJobs(ctx context.Context, query ImportJobListQuery) ([]domain.AudienceImportJob, string, error)
}

type ImportCounts struct {
	ProcessedCount int64
	CreatedCount   int64
	UpdatedCount   int64
	FailedCount    int64
}

type ImportJobWriteRepository interface {
	CreateImportJob(ctx context.Context, job domain.AudienceImportJob) error
	UpdateImportJob(ctx context.Context, job domain.AudienceImportJob) error
	ClaimQueuedImportJobs(ctx context.Context, limit int, now time.Time) ([]domain.AudienceImportJob, error)
	MarkImportJobRunning(ctx context.Context, workspaceID, jobID string, now time.Time) error
	MarkImportJobCompleted(ctx context.Context, workspaceID, jobID string, counts ImportCounts, now time.Time) error
	MarkImportJobFailed(ctx context.Context, workspaceID, jobID string, errorSummary string, now time.Time) error
}

type ExportJobListQuery struct {
	WorkspaceID string
	Status      string
	Limit       int
	Cursor      string
}

type ExportJobReadRepository interface {
	FindExportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error)
	ListExportJobs(ctx context.Context, query ExportJobListQuery) ([]domain.AudienceExportJob, string, error)
}

type ExportJobWriteRepository interface {
	CreateExportJob(ctx context.Context, job domain.AudienceExportJob) error
	UpdateExportJob(ctx context.Context, job domain.AudienceExportJob) error
	ClaimQueuedExportJobs(ctx context.Context, limit int, now time.Time) ([]domain.AudienceExportJob, error)
	MarkExportJobRunning(ctx context.Context, workspaceID, jobID string, now time.Time) error
	MarkExportJobCompleted(ctx context.Context, workspaceID, jobID string, artifactURI string, now time.Time) error
	MarkExportJobFailed(ctx context.Context, workspaceID, jobID string, errorSummary string, now time.Time) error
}

type OutboxEvent = outbox.Event

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}

type TransactionManager = transaction.UnitOfWork
