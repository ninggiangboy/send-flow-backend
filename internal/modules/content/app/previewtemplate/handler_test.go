package previewtemplate

import (
	"context"
	"errors"
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
					ID:         templateID,
					Subject:    "Hello {{.name}}",
					SourceHTML: "<p>Hello {{.name}}</p>",
				}, nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), Query{
		WorkspaceID:  "ws_1",
		TemplateID:   "tpl_1",
		UserID:       "user_1",
		TemplateData: map[string]any{"name": "Alice"},
	})
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

func TestExecute_MissingKey(t *testing.T) {
	h := New(Options{
		TemplatesRead: &mockTemplateRead{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{
					ID:         templateID,
					Subject:    "Hello {{.name}}",
					SourceHTML: "<p>Hello {{.name}}</p>",
				}, nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		Logger: testLogger(),
	})

	_, err := h.Execute(context.Background(), Query{
		WorkspaceID:  "ws_1",
		TemplateID:   "tpl_1",
		UserID:       "user_1",
		TemplateData: map[string]any{},
	})
	if !errors.Is(err, domain.ErrRenderPayloadInvalid) {
		t.Errorf("expected ErrRenderPayloadInvalid, got %v", err)
	}
}
