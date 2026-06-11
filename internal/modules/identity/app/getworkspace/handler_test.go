package getworkspace

import (
	"context"
	"errors"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type membershipsReadStub struct {
	membership *domain.Membership
	err        error
}

func (s *membershipsReadStub) FindByID(_ context.Context, _ string) (*domain.Membership, error) {
	return s.membership, s.err
}
func (s *membershipsReadStub) FindByWorkspaceAndUser(_ context.Context, _, _ string) (*domain.Membership, error) {
	return s.membership, s.err
}
func (s *membershipsReadStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Membership, error) {
	return nil, nil
}
func (s *membershipsReadStub) CountByWorkspaceAndRole(_ context.Context, _ string, _ domain.MembershipRole) (int, error) {
	return 0, nil
}

type workspacesReadStub struct {
	workspace *domain.Workspace
	err       error
}

func (s *workspacesReadStub) FindByID(_ context.Context, _ string) (*domain.Workspace, error) {
	return s.workspace, s.err
}
func (s *workspacesReadStub) ListByUser(_ context.Context, _ string) ([]domain.Workspace, error) {
	return nil, nil
}

func TestExecuteSuccess(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "u1"},
		},
		WorkspacesRead: &workspacesReadStub{
			workspace: &domain.Workspace{ID: "ws1", Name: "My Workspace"},
		},
	})

	ws, err := h.Execute(context.Background(), "ws1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.Name != "My Workspace" {
		t.Fatalf("expected 'My Workspace', got %s", ws.Name)
	}
}

func TestExecuteNonMember(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			err: domain.ErrMembershipNotFound,
		},
	})

	_, err := h.Execute(context.Background(), "ws1", "u1")
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got %v", err)
	}
}

func TestExecuteMembershipReadError(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			err: errors.New("db error"),
		},
	})

	_, err := h.Execute(context.Background(), "ws1", "u1")
	if err == nil || err.Error() != "db error" {
		t.Fatalf("expected 'db error', got %v", err)
	}
}
