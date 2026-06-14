package gettemplate

import (
	"context"
	"io"
	"log/slog"
	"testing"

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

type mockAccessChecker struct {
	requirePermission func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

func TestExecute(t *testing.T) {
	h := New(Options{
		TemplatesRead: &mockTemplateRead{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{
					ID:     templateID,
					Name:   "Test",
					Status: domain.TemplateStatusDraft,
				}, nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), "ws_1", "tpl_1", "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Template.ID != "tpl_1" {
		t.Errorf("expected ID tpl_1, got %s", result.Template.ID)
	}
}
