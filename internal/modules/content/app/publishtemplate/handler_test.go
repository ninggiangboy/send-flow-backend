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
	publish func(ctx context.Context, template domain.Template, version domain.TemplateVersion) error
}

func (m *mockTemplateWrite) PublishTemplateVersion(ctx context.Context, template domain.Template, version domain.TemplateVersion) error {
	return m.publish(ctx, template, version)
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
		TemplatesRead: &mockTemplateRead{
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
		},
		TemplatesWrite: &mockTemplateWrite{
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
		TemplatesRead: &mockTemplateRead{
			findByID: func(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
				return &domain.Template{
					ID:     templateID,
					Status: domain.TemplateStatusArchived,
				}, nil
			},
		},
		TemplatesWrite: &mockTemplateWrite{},
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
