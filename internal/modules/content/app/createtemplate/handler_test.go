package createtemplate

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

func TestExecute_CreatesTemplate(t *testing.T) {
	created := false
	h := New(Options{
		TemplatesWrite: &mockTemplateWrite{
			create: func(ctx context.Context, template domain.Template) error {
				created = true
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), Input{
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

func TestExecute_InvalidSource(t *testing.T) {
	h := New(Options{
		TemplatesWrite: &mockTemplateWrite{},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	_, err := h.Execute(context.Background(), Input{
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

func TestExecute_AccessDenied(t *testing.T) {
	h := New(Options{
		TemplatesWrite: &mockTemplateWrite{},
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error {
				return domain.ErrWriteDenied
			},
		},
		IDGen:  func() (string, error) { return "id_1", nil },
		Logger: testLogger(),
	})

	_, err := h.Execute(context.Background(), Input{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Name:        "Test",
		Subject:     "Hello {{.name}}",
		SourceHTML:  "<p>Hello</p>",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrWriteDenied) {
		t.Errorf("expected ErrWriteDenied, got %v", err)
	}
}
