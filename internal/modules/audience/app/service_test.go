package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type mockContactRead struct {
	ports.ContactReadRepository
	findByID    func(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error)
	findByEmail func(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error)
	list        func(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error)
}

func (m *mockContactRead) FindContactByID(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
	return m.findByID(ctx, workspaceID, contactID)
}

func (m *mockContactRead) FindContactByEmail(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
	return m.findByEmail(ctx, workspaceID, emailNormalized)
}

func (m *mockContactRead) ListContacts(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
	return m.list(ctx, query)
}

type mockContactWrite struct {
	ports.ContactWriteRepository
	create  func(ctx context.Context, contact domain.Contact) error
	update  func(ctx context.Context, contact domain.Contact) error
	archive func(ctx context.Context, workspaceID, contactID string, archivedAt time.Time) error
}

func (m *mockContactWrite) CreateContact(ctx context.Context, contact domain.Contact) error {
	return m.create(ctx, contact)
}

func (m *mockContactWrite) UpdateContact(ctx context.Context, contact domain.Contact) error {
	return m.update(ctx, contact)
}

func (m *mockContactWrite) ArchiveContact(ctx context.Context, workspaceID, contactID string, archivedAt time.Time) error {
	return m.archive(ctx, workspaceID, contactID, archivedAt)
}

type mockAccessChecker struct {
	ports.WorkspaceAccessChecker
	requirePermission func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

type mockListRead struct {
	ports.ListReadRepository
	findByID    func(ctx context.Context, workspaceID, listID string) (*domain.AudienceList, error)
	list        func(ctx context.Context, query ports.ListListQuery) ([]domain.AudienceList, string, error)
	countByList func(ctx context.Context, workspaceID string, listIDs []string) (map[string]int64, error)
}

func (m *mockListRead) FindListByID(ctx context.Context, workspaceID, listID string) (*domain.AudienceList, error) {
	return m.findByID(ctx, workspaceID, listID)
}

func (m *mockListRead) ListLists(ctx context.Context, query ports.ListListQuery) ([]domain.AudienceList, string, error) {
	return m.list(ctx, query)
}

func (m *mockListRead) CountContactsByList(ctx context.Context, workspaceID string, listIDs []string) (map[string]int64, error) {
	return m.countByList(ctx, workspaceID, listIDs)
}

type mockListWrite struct {
	ports.ListWriteRepository
	create  func(ctx context.Context, list domain.AudienceList) error
	replace func(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error)
	merge   func(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error)
}

func (m *mockListWrite) CreateList(ctx context.Context, list domain.AudienceList) error {
	return m.create(ctx, list)
}

func (m *mockListWrite) ReplaceListMemberships(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error) {
	return m.replace(ctx, workspaceID, listID, contactIDs, now)
}

func (m *mockListWrite) MergeListMemberships(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error) {
	return m.merge(ctx, workspaceID, listID, contactIDs, now)
}

type mockSegmentRead struct {
	ports.SegmentReadRepository
	findByID func(ctx context.Context, workspaceID, segmentID string) (*domain.Segment, error)
	list     func(ctx context.Context, query ports.SegmentListQuery) ([]domain.Segment, string, error)
}

func (m *mockSegmentRead) FindSegmentByID(ctx context.Context, workspaceID, segmentID string) (*domain.Segment, error) {
	return m.findByID(ctx, workspaceID, segmentID)
}

func (m *mockSegmentRead) ListSegments(ctx context.Context, query ports.SegmentListQuery) ([]domain.Segment, string, error) {
	return m.list(ctx, query)
}

type mockSegmentWrite struct {
	ports.SegmentWriteRepository
	create func(ctx context.Context, segment domain.Segment) error
	update func(ctx context.Context, segment domain.Segment) error
}

func (m *mockSegmentWrite) CreateSegment(ctx context.Context, segment domain.Segment) error {
	return m.create(ctx, segment)
}

func (m *mockSegmentWrite) UpdateSegment(ctx context.Context, segment domain.Segment) error {
	return m.update(ctx, segment)
}

type mockImportJobWrite struct {
	ports.ImportJobWriteRepository
	create func(ctx context.Context, job domain.AudienceImportJob) error
}

func (m *mockImportJobWrite) CreateImportJob(ctx context.Context, job domain.AudienceImportJob) error {
	return m.create(ctx, job)
}

type mockExportJobWrite struct {
	ports.ExportJobWriteRepository
	create func(ctx context.Context, job domain.AudienceExportJob) error
}

func (m *mockExportJobWrite) CreateExportJob(ctx context.Context, job domain.AudienceExportJob) error {
	return m.create(ctx, job)
}

func (m *mockExportJobWrite) UpdateExportJobProgress(ctx context.Context, workspaceID, jobID string, processedCount int64, now time.Time) error {
	return nil
}

type mockExportJobRead struct {
	ports.ExportJobReadRepository
	findByID func(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error)
	list     func(ctx context.Context, query ports.ExportJobListQuery) ([]domain.AudienceExportJob, string, error)
}

func (m *mockExportJobRead) FindExportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error) {
	return m.findByID(ctx, workspaceID, jobID)
}

func (m *mockExportJobRead) ListExportJobs(ctx context.Context, query ports.ExportJobListQuery) ([]domain.AudienceExportJob, string, error) {
	return m.list(ctx, query)
}

type mockArtifactSigner struct {
	presign func(ctx context.Context, key string, expiry time.Duration) (string, error)
}

func (m *mockArtifactSigner) PresignGetObject(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return m.presign(ctx, key, expiry)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCreateContact(t *testing.T) {
	created := false
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			findByEmail: func(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
				return nil, domain.ErrContactNotFound
			},
		},
		ContactsWrite: &mockContactWrite{
			create: func(ctx context.Context, contact domain.Contact) error {
				created = true
				if contact.EmailNormalized != "test@example.com" {
					t.Errorf("expected normalized email test@example.com, got %s", contact.EmailNormalized)
				}
				if contact.Status != domain.ContactStatusActive {
					t.Errorf("expected status active, got %s", contact.Status)
				}
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.CreateContact(context.Background(), CreateContactInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Email:       "test@example.com",
		FirstName:   "Test",
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected contact to be created")
	}
	if result.Contact.ID != "id_1" {
		t.Errorf("expected ID id_1, got %s", result.Contact.ID)
	}
}

func TestCreateContactEmailConflict(t *testing.T) {
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			findByEmail: func(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
				return &domain.Contact{ID: "existing", EmailNormalized: emailNormalized}, nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	_, err := svc.CreateContact(context.Background(), CreateContactInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Email:       "test@example.com",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrContactEmailConflict) {
		t.Errorf("expected ErrContactEmailConflict, got %v", err)
	}
}

func TestCreateContactPermissionDenied(t *testing.T) {
	svc := NewService(Options{
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error {
				return domain.ErrWriteDenied
			},
		},
		Logger: testLogger(),
	})

	_, err := svc.CreateContact(context.Background(), CreateContactInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Email:       "test@example.com",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrWriteDenied) {
		t.Errorf("expected ErrWriteDenied, got %v", err)
	}
}

func TestGetContact(t *testing.T) {
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			findByID: func(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
				return &domain.Contact{
					ID:     contactID,
					Email:  "test@example.com",
					Status: domain.ContactStatusActive,
				}, nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	result, err := svc.GetContact(context.Background(), "ws_1", "ct_1", "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Contact.ID != "ct_1" {
		t.Errorf("expected ID ct_1, got %s", result.Contact.ID)
	}
}

func TestArchiveContact(t *testing.T) {
	archived := false
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			findByID: func(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
				return &domain.Contact{ID: contactID}, nil
			},
		},
		ContactsWrite: &mockContactWrite{
			archive: func(ctx context.Context, workspaceID, contactID string, archivedAt time.Time) error {
				archived = true
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	err := svc.ArchiveContact(context.Background(), "ws_1", "ct_1", "user_1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !archived {
		t.Error("expected contact to be archived")
	}
}

func TestCreateList(t *testing.T) {
	created := false
	svc := NewService(Options{
		ListsWrite: &mockListWrite{
			create: func(ctx context.Context, list domain.AudienceList) error {
				created = true
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.CreateList(context.Background(), "ws_1", "user_1", "Test List", "", nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected list to be created")
	}
	if result.List.Name != "Test List" {
		t.Errorf("expected name 'Test List', got %s", result.List.Name)
	}
}

func TestCreateSegment(t *testing.T) {
	created := false
	svc := NewService(Options{
		SegmentsWrite: &mockSegmentWrite{
			create: func(ctx context.Context, segment domain.Segment) error {
				created = true
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.CreateSegment(context.Background(), "ws_1", "user_1", "Test Segment", map[string]any{"rules": []any{}}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected segment to be created")
	}
	if result.Segment.Name != "Test Segment" {
		t.Errorf("expected name 'Test Segment', got %s", result.Segment.Name)
	}
}

func TestCreateSegmentInvalidDefinition(t *testing.T) {
	svc := NewService(Options{
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	_, err := svc.CreateSegment(context.Background(), "ws_1", "user_1", "Test", nil, time.Now())
	if !errors.Is(err, domain.ErrSegmentDefinitionInvalid) {
		t.Errorf("expected ErrSegmentDefinitionInvalid, got %v", err)
	}
}

func TestStartAudienceImport(t *testing.T) {
	created := false
	svc := NewService(Options{
		ImportJobsWrite: &mockImportJobWrite{
			create: func(ctx context.Context, job domain.AudienceImportJob) error {
				created = true
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.StartAudienceImport(context.Background(), "ws_1", "user_1", "s3://bucket/file.csv", "by_email", nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected import job to be created")
	}
	if string(result.Job.DedupeMode) != "by_email" {
		t.Errorf("expected dedupe mode by_email, got %s", result.Job.DedupeMode)
	}
	if string(result.Job.Status) != "queued" {
		t.Errorf("expected status queued, got %s", result.Job.Status)
	}
}

func TestStartAudienceExport(t *testing.T) {
	created := false
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			list: func(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
				return []domain.Contact{
					{ID: "ct_1", Status: domain.ContactStatusActive},
					{ID: "ct_2", Status: domain.ContactStatusActive},
				}, "", nil
			},
		},
		SegmentsRead: &mockSegmentRead{
			findByID: func(ctx context.Context, workspaceID, segmentID string) (*domain.Segment, error) {
				return nil, domain.ErrSegmentNotFound
			},
			list: func(ctx context.Context, query ports.SegmentListQuery) ([]domain.Segment, string, error) {
				return nil, "", nil
			},
		},
		ExportJobsWrite: &mockExportJobWrite{
			create: func(ctx context.Context, job domain.AudienceExportJob) error {
				created = true
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		ExportEnabled: true,
		IDGen:         func() (string, error) { return "id_1", nil },
		Logger:        testLogger(),
	})

	result, err := svc.StartAudienceExport(context.Background(), "ws_1", "user_1", "csv", true, map[string]any{"list_id": "list_1"}, nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected export job to be created")
	}
	if string(result.Job.Format) != "csv" {
		t.Errorf("expected format csv, got %s", result.Job.Format)
	}
	if !result.Job.ZipOutput {
		t.Error("expected zip output to be persisted")
	}
	if result.Job.EstimatedTotalCount != 2 {
		t.Errorf("expected estimated total count 2, got %d", result.Job.EstimatedTotalCount)
	}
}

func TestStartAudienceExportUnavailable(t *testing.T) {
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			list: func(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
				return []domain.Contact{}, "", nil
			},
		},
		SegmentsRead: &mockSegmentRead{
			findByID: func(ctx context.Context, workspaceID, segmentID string) (*domain.Segment, error) {
				return nil, domain.ErrSegmentNotFound
			},
			list: func(ctx context.Context, query ports.SegmentListQuery) ([]domain.Segment, string, error) {
				return nil, "", nil
			},
		},
		ExportJobsWrite: &mockExportJobWrite{
			create: func(ctx context.Context, job domain.AudienceExportJob) error { return nil },
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	_, err := svc.StartAudienceExport(context.Background(), "ws_1", "user_1", "csv", false, nil, nil, time.Now())
	if !errors.Is(err, domain.ErrExportUnavailable) {
		t.Fatalf("expected ErrExportUnavailable, got %v", err)
	}
}

func TestGetAudienceExportAddsDownloadURL(t *testing.T) {
	svc := NewService(Options{
		ExportJobsRead: &mockExportJobRead{
			findByID: func(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error) {
				return &domain.AudienceExportJob{
					ID:                  jobID,
					WorkspaceID:         workspaceID,
					Format:              domain.ExportFormatCSV,
					Status:              domain.JobStatusCompleted,
					ProcessedCount:      10,
					EstimatedTotalCount: 12,
					ArtifactURI:         "exports/ws_1/job_1/job_1.csv",
				}, nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		ArtifactSigner: &mockArtifactSigner{
			presign: func(ctx context.Context, key string, expiry time.Duration) (string, error) {
				if key != "exports/ws_1/job_1/job_1.csv" {
					t.Fatalf("unexpected key %s", key)
				}
				return "https://download.example/export.csv", nil
			},
		},
		Logger: testLogger(),
	})

	result, err := svc.GetAudienceExport(context.Background(), "ws_1", "job_1", "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Job.DownloadURL == "" {
		t.Fatal("expected download URL to be populated")
	}
	if result.Job.DownloadURLTTL == nil {
		t.Fatal("expected download URL expiry to be populated")
	}
	if result.Job.EstimatedTotalCount != 12 {
		t.Fatalf("expected estimated total count 12, got %d", result.Job.EstimatedTotalCount)
	}
}

func TestResolveAudienceSelection(t *testing.T) {
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			list: func(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
				return []domain.Contact{
					{ID: "ct_1"},
					{ID: "ct_2"},
				}, "", nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	ids, err := svc.ResolveAudienceSelection(context.Background(), "ws_1", "user_1", AudienceSelectionRef{
		ListID: "list_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 contact IDs, got %d", len(ids))
	}
}

func TestEstimateAudienceSize(t *testing.T) {
	svc := NewService(Options{
		ContactsRead: &mockContactRead{
			list: func(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
				return []domain.Contact{
					{ID: "ct_1"},
					{ID: "ct_2"},
					{ID: "ct_3"},
				}, "", nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	size, err := svc.EstimateAudienceSize(context.Background(), "ws_1", "user_1", AudienceSelectionRef{
		ContactIDs: []string{"ct_1", "ct_2", "ct_3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 3 {
		t.Errorf("expected size 3, got %d", size)
	}
}
