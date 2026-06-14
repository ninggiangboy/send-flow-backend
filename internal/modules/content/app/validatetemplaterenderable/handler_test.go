package validatetemplaterenderable

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
		Logger: testLogger(),
	})

	err := h.Execute(context.Background(), "ws_1", "tpl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
