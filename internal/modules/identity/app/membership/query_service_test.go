package membership

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

func membershipTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type membershipReadStub struct {
	findByWorkspaceAndUser func(context.Context, string, string) (*domain.Membership, error)
	listByWorkspace        func(context.Context, string) ([]domain.Membership, error)
}

func (s *membershipReadStub) FindByID(context.Context, string) (*domain.Membership, error) {
	return nil, domain.ErrMembershipNotFound
}
func (s *membershipReadStub) FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	return s.findByWorkspaceAndUser(ctx, workspaceID, userID)
}
func (s *membershipReadStub) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Membership, error) {
	return s.listByWorkspace(ctx, workspaceID)
}
func (s *membershipReadStub) CountByWorkspaceAndRole(context.Context, string, domain.MembershipRole) (int, error) {
	return 0, nil
}

type invitationReadStub struct {
	listByWorkspace func(context.Context, string) ([]domain.Invitation, error)
}

func (s *invitationReadStub) FindByToken(context.Context, string) (*domain.Invitation, error) {
	return nil, domain.ErrInvitationNotFound
}
func (s *invitationReadStub) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Invitation, error) {
	return s.listByWorkspace(ctx, workspaceID)
}

func TestListMembersHandlerExecuteAccessDeniedForNonMember(t *testing.T) {
	h := NewListMembersHandler(&membershipReadStub{
		findByWorkspaceAndUser: func(context.Context, string, string) (*domain.Membership, error) {
			return nil, domain.ErrMembershipNotFound
		},
		listByWorkspace: func(context.Context, string) ([]domain.Membership, error) { return nil, nil },
	}, membershipTestLogger())

	_, err := h.Execute(context.Background(), "ws_1", "user_1")
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected access denied, got %v", err)
	}
}

func TestListInvitationsHandlerExecuteReturnsInvitations(t *testing.T) {
	h := NewListInvitationsHandler(
		&membershipReadStub{
			findByWorkspaceAndUser: func(context.Context, string, string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1"}, nil
			},
			listByWorkspace: func(context.Context, string) ([]domain.Membership, error) { return nil, nil },
		},
		&invitationReadStub{
			listByWorkspace: func(context.Context, string) ([]domain.Invitation, error) {
				return []domain.Invitation{{ID: "inv_1"}}, nil
			},
		},
		membershipTestLogger(),
	)

	invitations, err := h.Execute(context.Background(), "ws_1", "user_1")
	if err != nil || len(invitations) != 1 || invitations[0].ID != "inv_1" {
		t.Fatalf("unexpected result: %+v err=%v", invitations, err)
	}
}
