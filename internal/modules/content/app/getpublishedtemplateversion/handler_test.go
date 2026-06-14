package getpublishedtemplateversion

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
	findCurrent func(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error)
}

func (m *mockTemplateRead) FindCurrentVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
	return m.findCurrent(ctx, workspaceID, templateID)
}

func TestExecute(t *testing.T) {
	h := New(Options{
		TemplatesRead: &mockTemplateRead{
			findCurrent: func(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
				return &domain.TemplateVersion{
					ID:            "ver_1",
					TemplateID:    templateID,
					VersionNumber: 1,
				}, nil
			},
		},
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), "ws_1", "tpl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Version.ID != "ver_1" {
		t.Errorf("expected version ID ver_1, got %s", result.Version.ID)
	}
}
