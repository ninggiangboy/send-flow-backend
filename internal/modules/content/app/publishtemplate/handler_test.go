package publishtemplate

import (
	"context"
	"errors"
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
	findByID     func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error)
	listVersions func(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error)
}

func (m *mockTemplateRead) FindTemplateByID(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
	return m.findByID(ctx, workspaceID, templateID)
}

func (m *mockTemplateRead) ListTemplateVersions(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error) {
	return m.listVersions(ctx, workspaceID, templateID, cursor, limit)
}

type mockTemplateWrite struct {
	ports.TemplateWriteRepository
	publish         func(ctx context.Context, template domain.Template, version domain.TemplateVersion) error
	findByID        func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error)
	listTemplates   func(ctx context.Context, workspaceID, status, q, cursor string, limit int) ([]domain.Template, string, error)
	listVersions    func(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error)
	findVersionByID func(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error)
	findCurrentVer  func(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error)
}

func (m *mockTemplateWrite) PublishTemplateVersion(ctx context.Context, template domain.Template, version domain.TemplateVersion) error {
	return m.publish(ctx, template, version)
}

func (m *mockTemplateWrite) FindTemplateByID(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
	if m.findByID != nil {
		return m.findByID(ctx, workspaceID, templateID)
	}
	return nil, nil
}

func (m *mockTemplateWrite) ListTemplates(ctx context.Context, workspaceID, status, q, cursor string, limit int) ([]domain.Template, string, error) {
	if m.listTemplates != nil {
		return m.listTemplates(ctx, workspaceID, status, q, cursor, limit)
	}
	return nil, "", nil
}

func (m *mockTemplateWrite) ListTemplateVersions(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error) {
	if m.listVersions != nil {
		return m.listVersions(ctx, workspaceID, templateID, cursor, limit)
	}
	return nil, "", nil
}

func (m *mockTemplateWrite) FindTemplateVersionByID(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error) {
	if m.findVersionByID != nil {
		return m.findVersionByID(ctx, workspaceID, versionID)
	}
	return nil, nil
}

func (m *mockTemplateWrite) FindCurrentVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
	if m.findCurrentVer != nil {
		return m.findCurrentVer(ctx, workspaceID, templateID)
	}
	return nil, nil
}

type mockAccessChecker struct {
	requirePermission func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

func TestExecute_PublishesTemplate(t *testing.T) {
	published := false
	h := New(Options{
		TemplatesWrite: &mockTemplateWrite{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{
					ID:         templateID,
					Name:       "Test",
					Status:     domain.TemplateStatusDraft,
					Subject:    "Hello {{.name}}",
					SourceHTML: "<p>Hello {{.name}}</p>",
				}, nil
			},
			listVersions: func(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error) {
				return nil, "", nil
			},
			publish: func(ctx context.Context, template domain.Template, version domain.TemplateVersion) error {
				published = true
				if version.VersionNumber != 1 {
					t.Errorf("expected version number 1, got %d", version.VersionNumber)
				}
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "ver_1", nil },
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), "ws_1", "tpl_1", "user_1", time.Now())
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

func TestExecute_ArchivedTemplateFails(t *testing.T) {
	h := New(Options{
		TemplatesWrite: &mockTemplateWrite{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{
					ID:     templateID,
					Status: domain.TemplateStatusArchived,
				}, nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "ver_1", nil },
		Logger: testLogger(),
	})

	_, err := h.Execute(context.Background(), "ws_1", "tpl_1", "user_1", time.Now())
	if !errors.Is(err, domain.ErrPublishConflict) {
		t.Errorf("expected ErrPublishConflict, got %v", err)
	}
}
