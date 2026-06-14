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
	update func(ctx context.Context, template domain.Template) error
}

func (m *mockTemplateWrite) UpdateTemplate(ctx context.Context, template domain.Template) error {
	return m.update(ctx, template)
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
		TemplatesRead: &mockTemplateRead{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{
					ID:         templateID,
					Name:       "Original",
					Status:     domain.TemplateStatusDraft,
					Subject:    "Hello {{.name}}",
					SourceHTML: "<p>Hello {{.name}}</p>",
				}, nil
			},
		},
		TemplatesWrite: &mockTemplateWrite{
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
