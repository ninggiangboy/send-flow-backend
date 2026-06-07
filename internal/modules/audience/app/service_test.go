package app

import (
	"context"
	"errors"
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

type mockImportJobRead struct {
	ports.ImportJobReadRepository
	findByID func(ctx context.Context, workspaceID, jobID string) (*domain.AudienceImportJob, error)
	list     func(ctx context.Context, query ports.ImportJobListQuery) ([]domain.AudienceImportJob, string, error)
}

func (m *mockImportJobRead) FindImportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceImportJob, error) {
	return m.findByID(ctx, workspaceID, jobID)
}

func (m *mockImportJobRead) ListImportJobs(ctx context.Context, query ports.ImportJobListQuery) ([]domain.AudienceImportJob, string, error) {
	return m.list(ctx, query)
}

type mockImportJobWrite struct {
	ports.ImportJobWriteRepository
	create func(ctx context.Context, job domain.AudienceImportJob) error
}

func (m *mockImportJobWrite) CreateImportJob(ctx context.Context, job domain.AudienceImportJob) error {
	return m.create(ctx, job)
}

type mockExportJobRead struct {
	ports.ExportJobReadRepository
	findByID func(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error)
}

func (m *mockExportJobRead) FindExportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error) {
	return m.findByID(ctx, workspaceID, jobID)
}

type mockExportJobWrite struct {
	ports.ExportJobWriteRepository
	create func(ctx context.Context, job domain.AudienceExportJob) error
}

func (m *mockExportJobWrite) CreateExportJob(ctx context.Context, job domain.AudienceExportJob) error {
	return m.create(ctx, job)
}

func newServiceTestOpts() Options {
	return Options{
		ContactsRead:    &mockContactRead{},
		ContactsWrite:   &mockContactWrite{},
		ListsRead:       &mockListRead{},
		ListsWrite:      &mockListWrite{},
		SegmentsRead:    &mockSegmentRead{},
		SegmentsWrite:   &mockSegmentWrite{},
		ImportJobsRead:  &mockImportJobRead{},
		ImportJobsWrite: &mockImportJobWrite{},
		ExportJobsRead:  &mockExportJobRead{},
		ExportJobsWrite: &mockExportJobWrite{},
		IDGen:           func() (string, error) { return "id_1", nil },
		Logger:          slog.Default(),
	}
}

func TestCreateContact(t *testing.T) {
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	created := false
	write := opts.ContactsWrite.(*mockContactWrite)
	write.create = func(ctx context.Context, contact domain.Contact) error {
		created = true
		if contact.EmailNormalized != "test@example.com" {
			t.Errorf("expected normalized email test@example.com, got %s", contact.EmailNormalized)
		}
		if contact.Status != domain.ContactStatusActive {
			t.Errorf("expected status active, got %s", contact.Status)
		}
		return nil
	}
	read := opts.ContactsRead.(*mockContactRead)
	read.findByEmail = func(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
		return nil, domain.ErrContactNotFound
	}

	svc := NewService(opts)
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
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	read := opts.ContactsRead.(*mockContactRead)
	read.findByEmail = func(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
		return &domain.Contact{ID: "existing", EmailNormalized: emailNormalized}, nil
	}

	svc := NewService(opts)
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
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error {
		return domain.ErrWriteDenied
	}}
	opts.AccessChecker = access

	svc := NewService(opts)
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
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	read := opts.ContactsRead.(*mockContactRead)
	read.findByID = func(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
		return &domain.Contact{
			ID:     contactID,
			Email:  "test@example.com",
			Status: domain.ContactStatusActive,
		}, nil
	}

	svc := NewService(opts)
	result, err := svc.GetContact(context.Background(), "ws_1", "ct_1", "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Contact.ID != "ct_1" {
		t.Errorf("expected ID ct_1, got %s", result.Contact.ID)
	}
}

func TestArchiveContact(t *testing.T) {
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	archived := false
	read := opts.ContactsRead.(*mockContactRead)
	read.findByID = func(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
		return &domain.Contact{ID: contactID}, nil
	}
	write := opts.ContactsWrite.(*mockContactWrite)
	write.archive = func(ctx context.Context, workspaceID, contactID string, archivedAt time.Time) error {
		archived = true
		return nil
	}

	svc := NewService(opts)
	err := svc.ArchiveContact(context.Background(), "ws_1", "ct_1", "user_1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !archived {
		t.Error("expected contact to be archived")
	}
}

func TestCreateList(t *testing.T) {
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	created := false
	write := opts.ListsWrite.(*mockListWrite)
	write.create = func(ctx context.Context, list domain.AudienceList) error {
		created = true
		return nil
	}

	svc := NewService(opts)
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
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	created := false
	write := opts.SegmentsWrite.(*mockSegmentWrite)
	write.create = func(ctx context.Context, segment domain.Segment) error {
		created = true
		return nil
	}

	svc := NewService(opts)
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
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access

	svc := NewService(opts)
	_, err := svc.CreateSegment(context.Background(), "ws_1", "user_1", "Test", nil, time.Now())
	if !errors.Is(err, domain.ErrSegmentDefinitionInvalid) {
		t.Errorf("expected ErrSegmentDefinitionInvalid, got %v", err)
	}
}

func TestStartAudienceImport(t *testing.T) {
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	created := false
	write := opts.ImportJobsWrite.(*mockImportJobWrite)
	write.create = func(ctx context.Context, job domain.AudienceImportJob) error {
		created = true
		return nil
	}

	svc := NewService(opts)
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
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	created := false
	write := opts.ExportJobsWrite.(*mockExportJobWrite)
	write.create = func(ctx context.Context, job domain.AudienceExportJob) error {
		created = true
		return nil
	}

	svc := NewService(opts)
	result, err := svc.StartAudienceExport(context.Background(), "ws_1", "user_1", "csv", map[string]any{"list_id": "list_1"}, nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected export job to be created")
	}
	if string(result.Job.Format) != "csv" {
		t.Errorf("expected format csv, got %s", result.Job.Format)
	}
}

func TestResolveAudienceSelection(t *testing.T) {
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	read := opts.ContactsRead.(*mockContactRead)
	read.findByEmail = func(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
		return nil, domain.ErrContactNotFound
	}
	read.list = func(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
		return []domain.Contact{
			{ID: "ct_1"},
			{ID: "ct_2"},
		}, "", nil
	}

	svc := NewService(opts)
	ids, err := svc.ResolveAudienceSelection(context.Background(), "ws_1", AudienceSelectionRef{
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
	opts := newServiceTestOpts()
	access := &mockAccessChecker{requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil }}
	opts.AccessChecker = access
	read := opts.ContactsRead.(*mockContactRead)
	read.findByEmail = func(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
		return nil, domain.ErrContactNotFound
	}
	read.list = func(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
		return []domain.Contact{
			{ID: "ct_1"},
			{ID: "ct_2"},
			{ID: "ct_3"},
		}, "", nil
	}

	svc := NewService(opts)
	size, err := svc.EstimateAudienceSize(context.Background(), "ws_1", AudienceSelectionRef{
		ContactIDs: []string{"ct_1", "ct_2", "ct_3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 3 {
		t.Errorf("expected size 3, got %d", size)
	}
}
