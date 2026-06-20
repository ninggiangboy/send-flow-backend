package updatetemplate

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
	findByID func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error)
}

func (m *mockTemplateRead) FindTemplateByID(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
	return m.findByID(ctx, workspaceID, templateID)
}

type mockTemplateWrite struct {
	ports.TemplateWriteRepository
	update          func(ctx context.Context, template domain.Template) error
	findByID        func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error)
	listTemplates   func(ctx context.Context, workspaceID, status, q, cursor string, limit int) ([]domain.Template, string, error)
	listVersions    func(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error)
	findVersionByID func(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error)
	findCurrentVer  func(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error)
}

func (m *mockTemplateWrite) UpdateTemplate(ctx context.Context, template domain.Template) error {
	return m.update(ctx, template)
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

func TestExecute(t *testing.T) {
	updated := false
	h := New(Options{
		TemplatesWrite: &mockTemplateWrite{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{
					ID:         templateID,
					Name:       "Original",
					Status:     domain.TemplateStatusDraft,
					Subject:    "Hello {{.name}}",
					SourceHTML: "<p>Hello {{.name}}</p>",
				}, nil
			},
			update: func(ctx context.Context, template domain.Template) error {
				updated = true
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	newName := "Updated"
	result, err := h.Execute(context.Background(), Input{
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
