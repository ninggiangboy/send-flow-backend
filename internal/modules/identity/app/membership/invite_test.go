package membership

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type membershipUnitOfWorkStub struct{}

func (membershipUnitOfWorkStub) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type membershipIDGenStub struct{ id string }

func (s membershipIDGenStub) New() (string, error) { return s.id, nil }

type membershipsWriteStub struct {
	findByWorkspaceAndUser func(context.Context, string, string) (*domain.Membership, error)
}

func (s *membershipsWriteStub) Create(context.Context, domain.Membership) error { return nil }
func (s *membershipsWriteStub) DeleteByID(context.Context, string) error        { return nil }
func (s *membershipsWriteStub) UpdateRole(context.Context, string, domain.MembershipRole, time.Time) error {
	return nil
}
func (s *membershipsWriteStub) UpdateStatus(context.Context, string, domain.MembershipStatus, time.Time) error {
	return nil
}
func (s *membershipsWriteStub) FindByID(context.Context, string) (*domain.Membership, error) {
	return nil, nil
}
func (s *membershipsWriteStub) FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	if s.findByWorkspaceAndUser != nil {
		return s.findByWorkspaceAndUser(ctx, workspaceID, userID)
	}
	return nil, nil
}
func (s *membershipsWriteStub) ListByWorkspace(context.Context, string) ([]domain.Membership, error) {
	return nil, nil
}
func (s *membershipsWriteStub) CountByWorkspaceAndRole(context.Context, string, domain.MembershipRole) (int, error) {
	return 0, nil
}

type membershipRolesWriteStub struct {
	listByMembership func(context.Context, string) ([]domain.Role, error)
	findByIDs        func(context.Context, string, []string) ([]domain.Role, error)
}

func (s *membershipRolesWriteStub) Create(context.Context, domain.Role) error { return nil }
func (s *membershipRolesWriteStub) Update(context.Context, domain.Role) error { return nil }
func (s *membershipRolesWriteStub) ReplaceMembershipRoles(context.Context, string, []string, time.Time) error {
	return nil
}
func (s *membershipRolesWriteStub) ReplaceInvitationRoles(context.Context, string, []string, time.Time) error {
	return nil
}
func (s *membershipRolesWriteStub) FindByID(context.Context, string, string) (*domain.Role, error) {
	return nil, nil
}
func (s *membershipRolesWriteStub) FindByType(context.Context, string, domain.RoleType) (*domain.Role, error) {
	return nil, nil
}
func (s *membershipRolesWriteStub) FindByIDs(ctx context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error) {
	return s.findByIDs(ctx, workspaceID, roleIDs)
}
func (s *membershipRolesWriteStub) ListByWorkspace(context.Context, string) ([]domain.Role, error) {
	return nil, nil
}
func (s *membershipRolesWriteStub) ListByMembership(ctx context.Context, membershipID string) ([]domain.Role, error) {
	return s.listByMembership(ctx, membershipID)
}
func (s *membershipRolesWriteStub) ListByInvitation(context.Context, string) ([]domain.Role, error) {
	return nil, nil
}
func (s *membershipRolesWriteStub) CountMembershipsByRole(context.Context, string, string) (int, error) {
	return 0, nil
}

type membershipUserWriteStub struct {
	findByID    func(context.Context, string) (*domain.User, error)
	findByEmail func(context.Context, string) (*domain.User, error)
}

func (s *membershipUserWriteStub) Create(context.Context, domain.User) error { return nil }
func (s *membershipUserWriteStub) UpdatePassword(context.Context, string, string, time.Time) error {
	return nil
}
func (s *membershipUserWriteStub) MarkEmailVerified(context.Context, string, time.Time) error {
	return nil
}
func (s *membershipUserWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}
func (s *membershipUserWriteStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	return s.findByID(ctx, userID)
}
func (s *membershipUserWriteStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.findByEmail(ctx, email)
}

type membershipInvitationsWriteStub struct{}

func (membershipInvitationsWriteStub) Create(context.Context, domain.Invitation) error { return nil }
func (membershipInvitationsWriteStub) UpdateStatus(context.Context, string, domain.InvitationStatus, time.Time) error {
	return nil
}
func (membershipInvitationsWriteStub) FindByToken(context.Context, string) (*domain.Invitation, error) {
	return nil, nil
}
func (membershipInvitationsWriteStub) ListByWorkspace(context.Context, string) ([]domain.Invitation, error) {
	return nil, nil
}

type membershipOutboxStub struct{}

func (membershipOutboxStub) Save(context.Context, ports.OutboxEvent) error { return nil }

func TestInviteHandlerExecuteSuccess(t *testing.T) {
	h := NewInviteHandler(InviteOptions{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(context.Context, string, string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &membershipRolesWriteStub{
			listByMembership: func(context.Context, string) ([]domain.Role, error) {
				return []domain.Role{{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive}}, nil
			},
			findByIDs: func(context.Context, string, []string) ([]domain.Role, error) {
				return []domain.Role{{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive}}, nil
			},
		},
		UsersWrite: &membershipUserWriteStub{
			findByID: func(context.Context, string) (*domain.User, error) {
				return &domain.User{ID: "inviter1", Email: "inviter@example.com"}, nil
			},
			findByEmail: func(context.Context, string) (*domain.User, error) { return nil, domain.ErrNotFound },
		},
		InvitationsWrite: membershipInvitationsWriteStub{},
		IdGen:            membershipIDGenStub{id: "id1"},
		OutboxWriter:     membershipOutboxStub{},
		UnitOfWork:       membershipUnitOfWorkStub{},
		Logger:           membershipTestLogger(),
	})

	result, err := h.Execute(context.Background(), InviteCommand{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{"r1"},
		InviterID:   "inviter1",
		Now:         time.Now(),
	})
	if err != nil || result.Invitation == nil {
		t.Fatalf("unexpected result: %+v err=%v", result, err)
	}
}

func TestInviteHandlerExecuteInviterNotMember(t *testing.T) {
	h := NewInviteHandler(InviteOptions{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(context.Context, string, string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
			},
		},
		Logger: membershipTestLogger(),
	})

	_, err := h.Execute(context.Background(), InviteCommand{WorkspaceID: "ws1", Email: "invitee@example.com", RoleIDs: []string{"r1"}, InviterID: "inviter1", Now: time.Now()})
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got %v", err)
	}
}
