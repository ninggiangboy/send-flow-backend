package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

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

// TestServiceCompilation ensures the facade properly constructs and delegates.
func TestServiceCompilation(t *testing.T) {
	opts := Options{
		TemplatesRead: &mockTemplateRead{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{ID: templateID}, nil
			},
			list: func(ctx context.Context, workspaceID, status, q, cursor string, limit int) ([]domain.Template, string, error) {
				return nil, "", nil
			},
			listVersions: func(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error) {
				return nil, "", nil
			},
			findVersionByID: func(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error) {
				return &domain.TemplateVersion{}, nil
			},
			findCurrent: func(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
				return &domain.TemplateVersion{}, nil
			},
		},
		TemplatesWrite: &mockTemplateWrite{
			create: func(ctx context.Context, template domain.Template) error { return nil },
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	}

	svc := NewService(opts)
	if svc == nil {
		t.Fatal("expected service to be constructed")
	}

	// Verify facade methods compile and delegate without panicking.
	t.Run("CreateTemplate", func(t *testing.T) {
		result, err := svc.CreateTemplate(context.Background(), CreateTemplateInput{
			WorkspaceID: "ws_1",
			UserID:      "user_1",
			Name:        "Test",
			Subject:     "Hello {{.name}}",
			SourceHTML:  "<p>Hello</p>",
			Now:         time.Now(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Template.ID != "id_1" {
			t.Errorf("expected id_1, got %s", result.Template.ID)
		}
	})

	t.Run("GetPublishedTemplateVersion", func(t *testing.T) {
		version, err := svc.GetPublishedTemplateVersion(context.Background(), "ws_1", "tpl_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version == nil {
			t.Error("expected non-nil version")
		}
	})

	t.Run("ValidateTemplateRenderable", func(t *testing.T) {
		err := svc.ValidateTemplateRenderable(context.Background(), "ws_1", "tpl_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
