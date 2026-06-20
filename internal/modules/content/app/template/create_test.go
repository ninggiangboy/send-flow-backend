package template

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

func templateTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type templateWriteStub struct {
	create func(context.Context, domain.Template) error
}

func (m *templateWriteStub) CreateTemplate(ctx context.Context, template domain.Template) error {
	return m.create(ctx, template)
}
func (m *templateWriteStub) UpdateTemplate(context.Context, domain.Template) error { return nil }
func (m *templateWriteStub) PublishTemplateVersion(context.Context, domain.Template, domain.TemplateVersion) error {
	return nil
}
func (m *templateWriteStub) CreateRenderSnapshot(context.Context, domain.RenderedTemplateSnapshot) error {
	return nil
}
func (m *templateWriteStub) FindTemplateByID(context.Context, string, string) (*domain.Template, error) {
	return nil, domain.ErrTemplateNotFound
}
func (m *templateWriteStub) ListTemplates(context.Context, string, string, string, string, int) ([]domain.Template, string, error) {
	return nil, "", nil
}
func (m *templateWriteStub) ListTemplateVersions(context.Context, string, string, string, int) ([]domain.TemplateVersion, string, error) {
	return nil, "", nil
}
func (m *templateWriteStub) FindTemplateVersionByID(context.Context, string, string) (*domain.TemplateVersion, error) {
	return nil, domain.ErrTemplateVersionNotFound
}
func (m *templateWriteStub) FindCurrentVersion(context.Context, string, string) (*domain.TemplateVersion, error) {
	return nil, domain.ErrTemplateVersionNotFound
}

type templateAccessCheckerStub struct {
	requirePermission func(context.Context, string, string, string) error
}

func (m *templateAccessCheckerStub) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

func TestCreateHandlerExecuteCreatesTemplate(t *testing.T) {
	created := false
	h := NewCreateHandler(struct {
		TemplatesWrite ports.TemplateWriteRepository
		AccessChecker  ports.WorkspaceAccessChecker
		IDGen          func() (string, error)
		Logger         *slog.Logger
	}{
		TemplatesWrite: &templateWriteStub{create: func(context.Context, domain.Template) error { created = true; return nil }},
		AccessChecker:  &templateAccessCheckerStub{requirePermission: func(context.Context, string, string, string) error { return nil }},
		IDGen:          func() (string, error) { return "id_1", nil },
		Logger:         templateTestLogger(),
	})

	result, err := h.Execute(context.Background(), CreateInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Name:        "Test Template",
		Subject:     "Hello {{.name}}",
		SourceHTML:  "<p>Hello {{.name}}</p>",
		Now:         time.Now(),
	})
	if err != nil || !created || result.Template.Name != "Test Template" {
		t.Fatalf("unexpected result: %+v created=%v err=%v", result, created, err)
	}
}

func TestCreateHandlerExecuteAccessDenied(t *testing.T) {
	h := NewCreateHandler(struct {
		TemplatesWrite ports.TemplateWriteRepository
		AccessChecker  ports.WorkspaceAccessChecker
		IDGen          func() (string, error)
		Logger         *slog.Logger
	}{
		TemplatesWrite: &templateWriteStub{create: func(context.Context, domain.Template) error { return nil }},
		AccessChecker:  &templateAccessCheckerStub{requirePermission: func(context.Context, string, string, string) error { return domain.ErrWriteDenied }},
		IDGen:          func() (string, error) { return "id_1", nil },
		Logger:         templateTestLogger(),
	})

	_, err := h.Execute(context.Background(), CreateInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Name:        "Test",
		Subject:     "Hello {{.name}}",
		SourceHTML:  "<p>Hello</p>",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrWriteDenied) {
		t.Fatalf("expected ErrWriteDenied, got %v", err)
	}
}
