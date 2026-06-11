package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type mockTemplateRead struct {
	ports.TemplateReadRepository
	findByID        func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error)
	list            func(ctx context.Context, workspaceID, status, q, cursor string, limit int) ([]domain.Template, string, error)
	listVersions    func(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error)
	findVersionByID func(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error)
	findCurrent     func(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error)
}

func (m *mockTemplateRead) FindTemplateByID(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
	return m.findByID(ctx, workspaceID, templateID)
}

func (m *mockTemplateRead) ListTemplates(ctx context.Context, workspaceID, status, q, cursor string, limit int) ([]domain.Template, string, error) {
	return m.list(ctx, workspaceID, status, q, cursor, limit)
}

func (m *mockTemplateRead) ListTemplateVersions(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error) {
	return m.listVersions(ctx, workspaceID, templateID, cursor, limit)
}

func (m *mockTemplateRead) FindTemplateVersionByID(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error) {
	return m.findVersionByID(ctx, workspaceID, versionID)
}

func (m *mockTemplateRead) FindCurrentVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
	return m.findCurrent(ctx, workspaceID, templateID)
}

type mockTemplateWrite struct {
	ports.TemplateWriteRepository
	create   func(ctx context.Context, template domain.Template) error
	update   func(ctx context.Context, template domain.Template) error
	publish  func(ctx context.Context, template domain.Template, version domain.TemplateVersion) error
	snapshot func(ctx context.Context, snapshot domain.RenderedTemplateSnapshot) error
}

func (m *mockTemplateWrite) CreateTemplate(ctx context.Context, template domain.Template) error {
	return m.create(ctx, template)
}

func (m *mockTemplateWrite) UpdateTemplate(ctx context.Context, template domain.Template) error {
	return m.update(ctx, template)
}

func (m *mockTemplateWrite) PublishTemplateVersion(ctx context.Context, template domain.Template, version domain.TemplateVersion) error {
	return m.publish(ctx, template, version)
}

func (m *mockTemplateWrite) CreateRenderSnapshot(ctx context.Context, snapshot domain.RenderedTemplateSnapshot) error {
	return m.snapshot(ctx, snapshot)
}

type mockAccessChecker struct {
	requirePermission func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

func newContentTestOpts() Options {
	return Options{
		TemplatesRead:  &mockTemplateRead{},
		TemplatesWrite: &mockTemplateWrite{},
		IDGen:          func() (string, error) { return "id_1", nil },
		Logger:         slog.Default(),
	}
}

func TestCreateTemplate(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	created := false
	write := opts.TemplatesWrite.(*mockTemplateWrite)
	write.create = func(ctx context.Context, template domain.Template) error {
		created = true
		return nil
	}

	svc := NewService(opts)
	result, err := svc.CreateTemplate(context.Background(), CreateTemplateInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Name:        "Test Template",
		Subject:     "Hello {{.name}}",
		SourceHTML:  "<p>Hello {{.name}}</p>",
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected template to be created")
	}
	if result.Template.Name != "Test Template" {
		t.Errorf("expected name 'Test Template', got %s", result.Template.Name)
	}
	if string(result.Template.Status) != "draft" {
		t.Errorf("expected status draft, got %s", result.Template.Status)
	}
}

func TestCreateTemplateInvalidSource(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}

	svc := NewService(opts)
	_, err := svc.CreateTemplate(context.Background(), CreateTemplateInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Name:        "Test",
		Subject:     "Hello {{.name}",
		SourceHTML:  "<p>Hello</p>",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrSourceInvalid) {
		t.Errorf("expected ErrSourceInvalid, got %v", err)
	}
}

func TestGetTemplate(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findByID = func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
		return &domain.Template{
			ID:     templateID,
			Name:   "Test",
			Status: domain.TemplateStatusDraft,
		}, nil
	}

	svc := NewService(opts)
	result, err := svc.GetTemplate(context.Background(), "ws_1", "tpl_1", "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Template.ID != "tpl_1" {
		t.Errorf("expected ID tpl_1, got %s", result.Template.ID)
	}
}

func TestUpdateTemplate(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	updated := false
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findByID = func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
		return &domain.Template{
			ID:         templateID,
			Name:       "Original",
			Status:     domain.TemplateStatusDraft,
			Subject:    "Hello {{.name}}",
			SourceHTML: "<p>Hello {{.name}}</p>",
		}, nil
	}
	write := opts.TemplatesWrite.(*mockTemplateWrite)
	write.update = func(ctx context.Context, template domain.Template) error {
		updated = true
		return nil
	}

	newName := "Updated"
	svc := NewService(opts)
	result, err := svc.UpdateTemplate(context.Background(), UpdateTemplateInput{
		WorkspaceID: "ws_1",
		TemplateID:  "tpl_1",
		UserID:      "user_1",
		Name:        &newName,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected template to be updated")
	}
	if result.Template.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", result.Template.Name)
	}
}

func TestPublishTemplate(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	published := false
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findByID = func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
		return &domain.Template{
			ID:         templateID,
			Name:       "Test",
			Status:     domain.TemplateStatusDraft,
			Subject:    "Hello {{.name}}",
			SourceHTML: "<p>Hello {{.name}}</p>",
		}, nil
	}
	read.listVersions = func(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error) {
		return nil, "", nil
	}
	write := opts.TemplatesWrite.(*mockTemplateWrite)
	write.publish = func(ctx context.Context, template domain.Template, version domain.TemplateVersion) error {
		published = true
		if version.VersionNumber != 1 {
			t.Errorf("expected version number 1, got %d", version.VersionNumber)
		}
		return nil
	}

	svc := NewService(opts)
	result, err := svc.PublishTemplate(context.Background(), "ws_1", "tpl_1", "user_1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !published {
		t.Error("expected template to be published")
	}
	if result.Version.VersionNumber != 1 {
		t.Errorf("expected version number 1, got %d", result.Version.VersionNumber)
	}
}

func TestPublishTemplateArchived(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findByID = func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
		return &domain.Template{
			ID:     templateID,
			Status: domain.TemplateStatusArchived,
		}, nil
	}

	svc := NewService(opts)
	_, err := svc.PublishTemplate(context.Background(), "ws_1", "tpl_1", "user_1", time.Now())
	if !errors.Is(err, domain.ErrPublishConflict) {
		t.Errorf("expected ErrPublishConflict, got %v", err)
	}
}

func TestPreviewTemplate(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findByID = func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
		return &domain.Template{
			ID:         templateID,
			Subject:    "Hello {{.name}}",
			SourceHTML: "<p>Hello {{.name}}</p>",
		}, nil
	}

	svc := NewService(opts)
	result, err := svc.PreviewTemplate(context.Background(), "ws_1", "tpl_1", "user_1", map[string]any{"name": "Alice"}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Result.Subject != "Hello Alice" {
		t.Errorf("expected subject 'Hello Alice', got %s", result.Result.Subject)
	}
	if result.Result.HTML != "<p>Hello Alice</p>" {
		t.Errorf("expected HTML '<p>Hello Alice</p>', got %s", result.Result.HTML)
	}
}

func TestPreviewMissingKey(t *testing.T) {
	opts := newContentTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findByID = func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
		return &domain.Template{
			ID:         templateID,
			Subject:    "Hello {{.name}}",
			SourceHTML: "<p>Hello {{.name}}</p>",
		}, nil
	}

	svc := NewService(opts)
	_, err := svc.PreviewTemplate(context.Background(), "ws_1", "tpl_1", "user_1", map[string]any{}, time.Now())
	if !errors.Is(err, domain.ErrRenderPayloadInvalid) {
		t.Errorf("expected ErrRenderPayloadInvalid, got %v", err)
	}
}

func TestGetPublishedTemplateVersion(t *testing.T) {
	opts := newContentTestOpts()
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findCurrent = func(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
		return &domain.TemplateVersion{
			ID:            "ver_1",
			TemplateID:    templateID,
			VersionNumber: 1,
		}, nil
	}

	svc := NewService(opts)
	version, err := svc.GetPublishedTemplateVersion(context.Background(), "ws_1", "tpl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version.ID != "ver_1" {
		t.Errorf("expected version ID ver_1, got %s", version.ID)
	}
}

func TestValidateTemplateRenderable(t *testing.T) {
	opts := newContentTestOpts()
	read := opts.TemplatesRead.(*mockTemplateRead)
	read.findByID = func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
		return &domain.Template{
			ID:         templateID,
			Subject:    "Hello {{.name}}",
			SourceHTML: "<p>Hello {{.name}}</p>",
		}, nil
	}

	svc := NewService(opts)
	err := svc.ValidateTemplateRenderable(context.Background(), "ws_1", "tpl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
