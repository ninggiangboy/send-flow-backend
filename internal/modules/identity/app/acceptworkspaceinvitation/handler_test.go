package acceptworkspaceinvitation

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type noopTx struct{}

func (noopTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

var testLogger = slog.Default()

type idGenStub struct {
	id  string
	err error
}

func (s idGenStub) New() (string, error) { return s.id, s.err }

type invitationsReadStub struct {
	invitation *domain.Invitation
	err        error
}

func (s *invitationsReadStub) FindByToken(_ context.Context, _ string) (*domain.Invitation, error) {
	return s.invitation, s.err
}
func (s *invitationsReadStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Invitation, error) {
	return nil, nil
}

type usersReadStub struct {
	user *domain.User
	err  error
}

func (s *usersReadStub) FindByID(_ context.Context, _ string) (*domain.User, error) {
	return s.user, s.err
}
func (s *usersReadStub) FindByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, nil
}

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

type rolesReadStub struct {
	roles []domain.Role
	err   error
}

func (s *rolesReadStub) FindByID(_ context.Context, _, _ string) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) FindByType(_ context.Context, _ string, _ domain.RoleType) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) FindByIDs(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) ListByMembership(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) ListByInvitation(_ context.Context, _ string) ([]domain.Role, error) {
	return s.roles, s.err
}
func (s *rolesReadStub) CountMembershipsByRole(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

type membershipsWriteStub struct{}

func (s *membershipsWriteStub) Create(_ context.Context, _ domain.Membership) error { return nil }
func (s *membershipsWriteStub) DeleteByID(_ context.Context, _ string) error        { return nil }
func (s *membershipsWriteStub) UpdateRole(_ context.Context, _ string, _ domain.MembershipRole, _ time.Time) error {
	return nil
}
func (s *membershipsWriteStub) UpdateStatus(_ context.Context, _ string, _ domain.MembershipStatus, _ time.Time) error {
	return nil
}

type rolesWriteStub struct{}

func (s *rolesWriteStub) Create(_ context.Context, _ domain.Role) error { return nil }
func (s *rolesWriteStub) Update(_ context.Context, _ domain.Role) error { return nil }
func (s *rolesWriteStub) ReplaceMembershipRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return nil
}
func (s *rolesWriteStub) ReplaceInvitationRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return nil
}

type invitationsWriteStub struct{}

func (s *invitationsWriteStub) Create(_ context.Context, _ domain.Invitation) error { return nil }
func (s *invitationsWriteStub) UpdateStatus(_ context.Context, _ string, _ domain.InvitationStatus, _ time.Time) error {
	return nil
}

func TestExecuteSuccess(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "user@example.com", "token123", domain.MembershipRoleMember, now.Add(24*time.Hour), now)
	h := New(Options{
		InvitationsRead: &invitationsReadStub{invitation: &invitation},
		UsersRead:       &usersReadStub{user: &domain.User{ID: "u1", Email: "user@example.com"}},
		MembershipsRead: &membershipsReadStub{err: domain.ErrMembershipNotFound},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Name: "Member", Type: domain.RoleTypeMember, Status: domain.RoleStatusActive},
			},
		},
		MembershipsWrite: &membershipsWriteStub{},
		RolesWrite:       &rolesWriteStub{},
		InvitationsWrite: &invitationsWriteStub{},
		IdGen:            idGenStub{id: "m1"},
		UnitOfWork:       noopTx{},
		Logger:           testLogger,
	})

	membership, err := h.Execute(context.Background(), Command{Token: "token123", UserID: "u1", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if membership.ID != "m1" {
		t.Fatalf("expected membership ID 'm1', got %s", membership.ID)
	}
}

func TestExecuteInvalidToken(t *testing.T) {
	h := New(Options{
		InvitationsRead: &invitationsReadStub{err: domain.ErrInvitationNotFound},
		Logger:          testLogger,
	})

	_, err := h.Execute(context.Background(), Command{Token: "invalid", UserID: "u1", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Fatalf("expected ErrInvitationNotFound, got %v", err)
	}
}

func TestExecuteExpiredInvitation(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "user@example.com", "token123", domain.MembershipRoleMember, now.Add(-1*time.Hour), now)
	h := New(Options{
		InvitationsRead: &invitationsReadStub{invitation: &invitation},
		Logger:          testLogger,
	})

	_, err := h.Execute(context.Background(), Command{Token: "token123", UserID: "u1", Now: now})
	if !errors.Is(err, domain.ErrInvitationExpired) {
		t.Fatalf("expected ErrInvitationExpired, got %v", err)
	}
}

func TestExecuteAlreadyAccepted(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "user@example.com", "token123", domain.MembershipRoleMember, now.Add(24*time.Hour), now)
	invitation.Status = domain.InvitationStatusAccepted
	h := New(Options{
		InvitationsRead: &invitationsReadStub{invitation: &invitation},
		Logger:          testLogger,
	})

	_, err := h.Execute(context.Background(), Command{Token: "token123", UserID: "u1", Now: now})
	if !errors.Is(err, domain.ErrInvitationAccepted) {
		t.Fatalf("expected ErrInvitationAccepted, got %v", err)
	}
}

func TestExecuteEmailMismatch(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "other@example.com", "token123", domain.MembershipRoleMember, now.Add(24*time.Hour), now)
	h := New(Options{
		InvitationsRead: &invitationsReadStub{invitation: &invitation},
		UsersRead:       &usersReadStub{user: &domain.User{ID: "u1", Email: "user@example.com"}},
		Logger:          testLogger,
	})

	_, err := h.Execute(context.Background(), Command{Token: "token123", UserID: "u1", Now: now})
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got %v", err)
	}
}

func TestExecuteAlreadyMember(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "user@example.com", "token123", domain.MembershipRoleMember, now.Add(24*time.Hour), now)
	h := New(Options{
		InvitationsRead: &invitationsReadStub{invitation: &invitation},
		UsersRead:       &usersReadStub{user: &domain.User{ID: "u1", Email: "user@example.com"}},
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "u1"},
		},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{Token: "token123", UserID: "u1", Now: now})
	if !errors.Is(err, domain.ErrInvitationAccepted) {
		t.Fatalf("expected ErrInvitationAccepted, got %v", err)
	}
}

func TestExecuteInvitationNoRoles(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "user@example.com", "token123", domain.MembershipRoleMember, now.Add(24*time.Hour), now)
	h := New(Options{
		InvitationsRead: &invitationsReadStub{invitation: &invitation},
		UsersRead:       &usersReadStub{user: &domain.User{ID: "u1", Email: "user@example.com"}},
		MembershipsRead: &membershipsReadStub{err: domain.ErrMembershipNotFound},
		RolesRead:       &rolesReadStub{roles: []domain.Role{}},
		Logger:          testLogger,
	})

	_, err := h.Execute(context.Background(), Command{Token: "token123", UserID: "u1", Now: now})
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestExecuteIDGenError(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "user@example.com", "token123", domain.MembershipRoleMember, now.Add(24*time.Hour), now)
	h := New(Options{
		InvitationsRead: &invitationsReadStub{invitation: &invitation},
		UsersRead:       &usersReadStub{user: &domain.User{ID: "u1", Email: "user@example.com"}},
		MembershipsRead: &membershipsReadStub{err: domain.ErrMembershipNotFound},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Name: "Member", Type: domain.RoleTypeMember, Status: domain.RoleStatusActive},
			},
		},
		IdGen:  idGenStub{err: errors.New("idgen error")},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{Token: "token123", UserID: "u1", Now: now})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
