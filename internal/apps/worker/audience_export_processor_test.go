package worker

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	audienceports "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
)

type exportJobRepoStub struct {
	completedArtifact string
	failedSummary     string
	processedCounts   []int64
}

func (s *exportJobRepoStub) CreateExportJob(ctx context.Context, job domain.AudienceExportJob) error {
	return nil
}

func (s *exportJobRepoStub) UpdateExportJob(ctx context.Context, job domain.AudienceExportJob) error {
	return nil
}

func (s *exportJobRepoStub) ClaimQueuedExportJobs(ctx context.Context, limit int, now time.Time) ([]domain.AudienceExportJob, error) {
	return nil, nil
}

func (s *exportJobRepoStub) MarkExportJobRunning(ctx context.Context, workspaceID, jobID string, now time.Time) error {
	return nil
}

func (s *exportJobRepoStub) UpdateExportJobProgress(ctx context.Context, workspaceID, jobID string, processedCount int64, now time.Time) error {
	s.processedCounts = append(s.processedCounts, processedCount)
	return nil
}

func (s *exportJobRepoStub) MarkExportJobCompleted(ctx context.Context, workspaceID, jobID string, artifactURI string, now time.Time) error {
	s.completedArtifact = artifactURI
	return nil
}

func (s *exportJobRepoStub) MarkExportJobFailed(ctx context.Context, workspaceID, jobID string, errorSummary string, now time.Time) error {
	s.failedSummary = errorSummary
	return nil
}

func (s *exportJobRepoStub) FindExportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error) {
	return nil, nil
}

func (s *exportJobRepoStub) ListExportJobs(ctx context.Context, query audienceports.ExportJobListQuery) ([]domain.AudienceExportJob, string, error) {
	return nil, "", nil
}

type exportContactRepoStub struct {
	contacts []domain.Contact
}

func (s *exportContactRepoStub) FindContactByID(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
	return nil, domain.ErrContactNotFound
}

func (s *exportContactRepoStub) FindContactByEmail(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
	return nil, domain.ErrContactNotFound
}

func (s *exportContactRepoStub) ListContacts(ctx context.Context, query audienceports.ContactListQuery) ([]domain.Contact, string, error) {
	if query.Cursor != "" {
		return []domain.Contact{}, "", nil
	}
	return s.contacts, "", nil
}

type exportSegmentRepoStub struct {
	segment *domain.Segment
	err     error
}

func (s *exportSegmentRepoStub) FindSegmentByID(ctx context.Context, workspaceID, segmentID string) (*domain.Segment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.segment, nil
}

func (s *exportSegmentRepoStub) ListSegments(ctx context.Context, query audienceports.SegmentListQuery) ([]domain.Segment, string, error) {
	return nil, "", nil
}

type exportOutboxStub struct {
	events []outbox.Event
}

func (s *exportOutboxStub) Save(ctx context.Context, event audienceports.OutboxEvent) error {
	s.events = append(s.events, event)
	return nil
}

type exportTxStub struct{}

func (s *exportTxStub) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type exportStorageStub struct {
	putErr      error
	content     string
	contentType string
}

func (s *exportStorageStub) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.contentType = contentType
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.content = string(data)
	return nil
}

func (s *exportStorageStub) DeleteObject(ctx context.Context, key string) error {
	return nil
}

func (s *exportStorageStub) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(s.content)), nil
}

func (s *exportStorageStub) PresignGetObject(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "", nil
}

func (s *exportStorageStub) EnsureBucket(ctx context.Context) error {
	return nil
}

func (s *exportStorageStub) Ping(ctx context.Context) error {
	return nil
}

func TestAudienceExportProcessorProcessJobFiltersBySegment(t *testing.T) {
	jobRepo := &exportJobRepoStub{}
	storage := &exportStorageStub{}
	outboxWriter := &exportOutboxStub{}
	processor := &AudienceExportProcessor{
		exportJobsWrite: jobRepo,
		contactsRead: &exportContactRepoStub{
			contacts: []domain.Contact{
				{ID: "contact-1", Email: "vip@example.com", FirstName: "VIP", Status: domain.ContactStatusActive, Tags: []string{"vip"}},
				{ID: "contact-2", Email: "plain@example.com", FirstName: "Plain", Status: domain.ContactStatusActive, Tags: []string{"basic"}},
			},
		},
		segmentsRead: &exportSegmentRepoStub{
			segment: &domain.Segment{
				ID:          "segment-1",
				WorkspaceID: "workspace-1",
				Status:      domain.SegmentStatusReady,
				DefinitionJSON: map[string]any{
					"rules": []any{
						map[string]any{"field": "tags", "operator": "contains", "value": "vip"},
					},
				},
			},
		},
		outboxWriter: outboxWriter,
		txManager:    &exportTxStub{},
		objStorage:   storage,
		idGen: func() (string, error) {
			return "event-1", nil
		},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	job := domain.AudienceExportJob{
		ID:             "job-1",
		WorkspaceID:    "workspace-1",
		Format:         domain.ExportFormatCSV,
		Status:         domain.JobStatusRunning,
		FiltersJSON:    map[string]any{"segment_id": "segment-1", "list_id": "list-1"},
		SelectedFields: []string{"email", "first_name"},
	}

	if err := processor.processJob(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(storage.content, "vip@example.com,VIP") {
		t.Fatalf("expected VIP contact in export, got %q", storage.content)
	}
	if strings.Contains(storage.content, "plain@example.com") {
		t.Fatalf("did not expect non-matching contact in export, got %q", storage.content)
	}
	if jobRepo.completedArtifact == "" {
		t.Fatal("expected completed artifact to be recorded")
	}
	if len(jobRepo.processedCounts) == 0 || jobRepo.processedCounts[len(jobRepo.processedCounts)-1] != 1 {
		t.Fatalf("expected processed count updates, got %+v", jobRepo.processedCounts)
	}
	if len(outboxWriter.events) != 1 || outboxWriter.events[0].EventType == "" {
		t.Fatalf("expected completed outbox event, got %+v", outboxWriter.events)
	}
}

func TestAudienceExportProcessorProcessJobPublishesFailureEvent(t *testing.T) {
	jobRepo := &exportJobRepoStub{}
	outboxWriter := &exportOutboxStub{}
	processor := &AudienceExportProcessor{
		exportJobsWrite: jobRepo,
		contactsRead: &exportContactRepoStub{
			contacts: []domain.Contact{
				{ID: "contact-1", Email: "vip@example.com", Status: domain.ContactStatusActive},
			},
		},
		segmentsRead: &exportSegmentRepoStub{},
		outboxWriter: outboxWriter,
		txManager:    &exportTxStub{},
		objStorage:   &exportStorageStub{putErr: errors.New("storage down")},
		idGen: func() (string, error) {
			return "event-2", nil
		},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	job := domain.AudienceExportJob{
		ID:          "job-2",
		WorkspaceID: "workspace-1",
		Format:      domain.ExportFormatJSON,
		Status:      domain.JobStatusRunning,
	}

	err := processor.processJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(jobRepo.failedSummary, "failed to store artifact") {
		t.Fatalf("expected failed summary to be recorded, got %q", jobRepo.failedSummary)
	}
	if len(outboxWriter.events) != 1 {
		t.Fatalf("expected one failure event, got %d", len(outboxWriter.events))
	}
	if outboxWriter.events[0].EventType != "audience.export.failed.v1" {
		t.Fatalf("expected failure event type, got %s", outboxWriter.events[0].EventType)
	}
}

func TestAudienceExportProcessorWriteJSONStreamProducesArray(t *testing.T) {
	processor := &AudienceExportProcessor{
		contactsRead: &exportContactRepoStub{
			contacts: []domain.Contact{
				{ID: "contact-1", Email: "vip@example.com", Status: domain.ContactStatusActive, Tags: []string{"vip"}},
			},
		},
	}

	var buf bytes.Buffer
	count, err := processor.writeJSONStream(context.Background(), &buf, domain.AudienceExportJob{
		SelectedFields: []string{"email", "tags"},
	}, audienceports.ContactListQuery{
		WorkspaceID: "workspace-1",
		Limit:       1000,
	}, nil, func(int) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
	if got := strings.TrimSpace(buf.String()); got != "[{\"email\":\"vip@example.com\",\"tags\":[\"vip\"]}\n]" {
		t.Fatalf("unexpected JSON payload %q", got)
	}
}

func TestAudienceExportProcessorProcessJobWritesZipWithBOM(t *testing.T) {
	jobRepo := &exportJobRepoStub{}
	storage := &exportStorageStub{}
	processor := &AudienceExportProcessor{
		exportJobsWrite: jobRepo,
		contactsRead: &exportContactRepoStub{
			contacts: []domain.Contact{
				{ID: "contact-1", Email: "vip@example.com", FirstName: "VIP", Status: domain.ContactStatusActive},
			},
		},
		segmentsRead: &exportSegmentRepoStub{},
		outboxWriter: &exportOutboxStub{},
		txManager:    &exportTxStub{},
		objStorage:   storage,
		idGen: func() (string, error) {
			return "event-3", nil
		},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	job := domain.AudienceExportJob{
		ID:             "job-3",
		WorkspaceID:    "workspace-1",
		Format:         domain.ExportFormatCSV,
		ZipOutput:      true,
		Status:         domain.JobStatusRunning,
		SelectedFields: []string{"email", "first_name"},
	}

	if err := processor.processJob(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.contentType != "application/zip" {
		t.Fatalf("expected zip content type, got %q", storage.contentType)
	}

	reader, err := zip.NewReader(bytes.NewReader([]byte(storage.content)), int64(len(storage.content)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(reader.File) != 1 {
		t.Fatalf("expected one zip entry, got %d", len(reader.File))
	}
	rc, err := reader.File[0].Open()
	if err != nil {
		t.Fatalf("open zip entry: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read zip entry: %v", err)
	}
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("expected UTF-8 BOM, got %q", data[:min(3, len(data))])
	}
	if !strings.Contains(string(data), "vip@example.com,VIP") {
		t.Fatalf("expected CSV content in zip entry, got %q", string(data))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
