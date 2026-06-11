package listworkspaceinvitations

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var testLogger = slog.Default()

type membershipsReadStub struct {
	findByWorkspaceAndUser func(ctx context.Context, workspaceID, userID string) (*domain.Membership, error)
}

func (s *membershipsReadStub) FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	return s.findByWorkspaceAndUser(ctx, workspaceID, userID)
}
func (s *membershipsReadStub) FindByID(ctx context.Context, membershipID string) (*domain.Membership, error) {
	return nil, nil
}
func (s *membershipsReadStub) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Membership, error) {
	return nil, nil
}
func (s *membershipsReadStub) CountByWorkspaceAndRole(ctx context.Context, workspaceID string, role domain.MembershipRole) (int, error) {
	return 0, nil
}

type invitationsReadStub struct {
	listByWorkspace func(ctx context.Context, workspaceID string) ([]domain.Invitation, error)
}

func (s *invitationsReadStub) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Invitation, error) {
	return s.listByWorkspace(ctx, workspaceID)
}
func (s *invitationsReadStub) FindByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	return nil, nil
}

func TestListWorkspaceInvitationsSuccess(t *testing.T) {
	invitations := []domain.Invitation{{ID: "inv-1", WorkspaceID: "ws-1", Email: "a@example.com"}}
	membership := &domain.Membership{ID: "mem-1", WorkspaceID: "ws-1", UserID: "u1"}
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return membership, nil
			},
		},
		InvitationsRead: &invitationsReadStub{
			listByWorkspace: func(_ context.Context, _ string) ([]domain.Invitation, error) {
				return invitations, nil
			},
		},
	})
	result, err := h.Execute(context.Background(), "ws-1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].ID != "inv-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestListWorkspaceInvitations_NotMember(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
			},
		},
	})
	_, err := h.Execute(context.Background(), "ws-1", "u1")
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got: %v", err)
	}
}

func TestListWorkspaceInvitations_MembershipReadError(t *testing.T) {
	expectedErr := errors.New("db error")
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, expectedErr
			},
		},
	})
	_, err := h.Execute(context.Background(), "ws-1", "u1")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}

func TestListWorkspaceInvitations_InvitationsReadError(t *testing.T) {
	expectedErr := errors.New("invitations error")
	membership := &domain.Membership{ID: "mem-1", WorkspaceID: "ws-1", UserID: "u1"}
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return membership, nil
			},
		},
		InvitationsRead: &invitationsReadStub{
			listByWorkspace: func(_ context.Context, _ string) ([]domain.Invitation, error) {
				return nil, expectedErr
			},
		},
	})
	_, err := h.Execute(context.Background(), "ws-1", "u1")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}
