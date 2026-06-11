package listworkspaces

import (
	"context"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type workspacesReadStub struct {
	workspaces []domain.Workspace
	err        error
}

func (s *workspacesReadStub) FindByID(_ context.Context, _ string) (*domain.Workspace, error) {
	return nil, nil
}
func (s *workspacesReadStub) ListByUser(_ context.Context, _ string) ([]domain.Workspace, error) {
	return s.workspaces, s.err
}

func TestExecuteSuccess(t *testing.T) {
	h := New(usecase.Deps{
		WorkspacesRead: &workspacesReadStub{
			workspaces: []domain.Workspace{
				{ID: "ws1", Name: "Workspace 1"},
				{ID: "ws2", Name: "Workspace 2"},
			},
		},
	})

	result, err := h.Execute(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(result))
	}
}

func TestExecuteEmpty(t *testing.T) {
	h := New(usecase.Deps{
		WorkspacesRead: &workspacesReadStub{
			workspaces: []domain.Workspace{},
		},
	})

	result, err := h.Execute(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 workspaces, got %d", len(result))
	}
}

func TestExecuteError(t *testing.T) {
	h := New(usecase.Deps{
		WorkspacesRead: &workspacesReadStub{
			err: domain.ErrWorkspaceNotFound,
		},
	})

	_, err := h.Execute(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
