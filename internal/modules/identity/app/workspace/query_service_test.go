package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type workspaceReadStub struct {
	findByID   func(context.Context, string) (*domain.Workspace, error)
	listByUser func(context.Context, string) ([]domain.Workspace, error)
}

func (s *workspaceReadStub) Create(context.Context, domain.Workspace) error { return nil }
func (s *workspaceReadStub) FindByID(ctx context.Context, workspaceID string) (*domain.Workspace, error) {
	return s.findByID(ctx, workspaceID)
}
func (s *workspaceReadStub) ListByUser(ctx context.Context, userID string) ([]domain.Workspace, error) {
	return s.listByUser(ctx, userID)
}

type workspaceMembershipReadStub struct {
	findByWorkspaceAndUser func(context.Context, string, string) (*domain.Membership, error)
}

func (s *workspaceMembershipReadStub) FindByID(context.Context, string) (*domain.Membership, error) {
	return nil, domain.ErrMembershipNotFound
}
func (s *workspaceMembershipReadStub) FindByWorkspaceAndUser(ctx context.Context, wsID, userID string) (*domain.Membership, error) {
	return s.findByWorkspaceAndUser(ctx, wsID, userID)
}
func (s *workspaceMembershipReadStub) ListByWorkspace(context.Context, string) ([]domain.Membership, error) {
	return nil, nil
}
func (s *workspaceMembershipReadStub) CountByWorkspaceAndRole(context.Context, string, domain.MembershipRole) (int, error) {
	return 0, nil
}

func TestGetHandlerExecuteDeniesNonMember(t *testing.T) {
	h := NewGetHandler(&workspaceReadStub{}, &workspaceMembershipReadStub{
		findByWorkspaceAndUser: func(context.Context, string, string) (*domain.Membership, error) {
			return nil, domain.ErrMembershipNotFound
		},
	}, workspaceTestLogger())

	_, err := h.Execute(context.Background(), "ws_1", "user_1")
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected access denied, got %v", err)
	}
}

func TestListHandlerExecuteReturnsWorkspaces(t *testing.T) {
	h := NewListHandler(&workspaceReadStub{
		listByUser: func(context.Context, string) ([]domain.Workspace, error) {
			return []domain.Workspace{{ID: "ws_1"}}, nil
		},
	}, workspaceTestLogger())

	workspaces, err := h.Execute(context.Background(), "user_1")
	if err != nil || len(workspaces) != 1 || workspaces[0].ID != "ws_1" {
		t.Fatalf("unexpected result: %+v err=%v", workspaces, err)
	}
}
