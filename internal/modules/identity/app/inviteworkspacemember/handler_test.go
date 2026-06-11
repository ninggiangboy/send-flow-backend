package inviteworkspacemember

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type noopTx struct{}

func (noopTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

var testLogger = slog.Default()

type idGenStub struct {
	id string
}

func (s idGenStub) New() (string, error) { return s.id, nil }

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
	roles         []domain.Role
	listMemberErr error
	findByIDsRes  []domain.Role
	findByIDsErr  error
	listInviteErr error
}

func (s *rolesReadStub) FindByID(_ context.Context, _, _ string) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) FindByType(_ context.Context, _ string, _ domain.RoleType) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) FindByIDs(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
	return s.findByIDsRes, s.findByIDsErr
}
func (s *rolesReadStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) ListByMembership(_ context.Context, _ string) ([]domain.Role, error) {
	return s.roles, s.listMemberErr
}
func (s *rolesReadStub) ListByInvitation(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, s.listInviteErr
}
func (s *rolesReadStub) CountMembershipsByRole(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

type usersReadStub struct {
	user           *domain.User
	findByIDErr    error
	findByEmailRes *domain.User
	findByEmailErr error
}

func (s *usersReadStub) FindByID(_ context.Context, _ string) (*domain.User, error) {
	return s.user, s.findByIDErr
}
func (s *usersReadStub) FindByEmail(_ context.Context, _ string) (*domain.User, error) {
	return s.findByEmailRes, s.findByEmailErr
}

type invitationsWriteStub struct{}

func (s *invitationsWriteStub) Create(_ context.Context, _ domain.Invitation) error { return nil }
func (s *invitationsWriteStub) UpdateStatus(_ context.Context, _ string, _ domain.InvitationStatus, _ time.Time) error {
	return nil
}

type rolesWriteStub struct {
	createErr        error
	replaceMemberErr error
	replaceInviteErr error
}

func (s *rolesWriteStub) Create(_ context.Context, _ domain.Role) error { return s.createErr }
func (s *rolesWriteStub) Update(_ context.Context, _ domain.Role) error { return nil }
func (s *rolesWriteStub) ReplaceMembershipRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return s.replaceMemberErr
}
func (s *rolesWriteStub) ReplaceInvitationRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return s.replaceInviteErr
}

func TestExecuteSuccess(t *testing.T) {
	now := time.Now().UTC()
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"},
		},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
			findByIDsRes: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
		},
		UsersRead: &usersReadStub{
			user:           &domain.User{ID: "inviter1", Email: "inviter@example.com"},
			findByEmailRes: nil,
			findByEmailErr: domain.ErrNotFound,
		},
		InvitationsWrite: &invitationsWriteStub{},
		RolesWrite:       &rolesWriteStub{},
		IDGen:            idGenStub{id: "id1"},
		UnitOfWork:       noopTx{},
		Logger:           testLogger,
	})

	result, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{"r1"},
		InviterID:   "inviter1",
		Now:         now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Invitation == nil {
		t.Fatal("expected invitation in result")
	}
}

func TestExecuteInviterNotMember(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{err: domain.ErrMembershipNotFound},
		Logger:          testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{"r1"},
		InviterID:   "inviter1",
		Now:         time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got %v", err)
	}
}

func TestExecuteInviterLacksPermission(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"},
		},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeMember, PermissionsMask: 0, Status: domain.RoleStatusActive},
			},
		},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{"r1"},
		InviterID:   "inviter1",
		Now:         time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrMembershipManageDenied) {
		t.Fatalf("expected ErrMembershipManageDenied, got %v", err)
	}
}

func TestExecuteNoRoles(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"},
		},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
		},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{},
		InviterID:   "inviter1",
		Now:         time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrInvitationPayloadInvalid) {
		t.Fatalf("expected ErrInvitationPayloadInvalid, got %v", err)
	}
}

func TestExecuteInvalidRoleIDs(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"},
		},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
			findByIDsRes: []domain.Role{},
		},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{"invalid-role"},
		InviterID:   "inviter1",
		Now:         time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestExecuteRoleInactive(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"},
		},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
			findByIDsRes: []domain.Role{
				{ID: "r2", Type: domain.RoleTypeMember, PermissionsMask: 0, Status: domain.RoleStatusArchived},
			},
		},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{"r2"},
		InviterID:   "inviter1",
		Now:         time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestExecuteEmptyEmail(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"},
		},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
			findByIDsRes: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
		},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "  ",
		RoleIDs:     []string{"r1"},
		InviterID:   "inviter1",
		Now:         time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrInvitationPayloadInvalid) {
		t.Fatalf("expected ErrInvitationPayloadInvalid, got %v", err)
	}
}

func TestExecuteUserAlreadyMember(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{
			membership: &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"},
		},
		RolesRead: &rolesReadStub{
			roles: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
			findByIDsRes: []domain.Role{
				{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
			},
		},
		UsersRead: &usersReadStub{
			user:           &domain.User{ID: "inviter1", Email: "inviter@example.com"},
			findByEmailRes: &domain.User{ID: "existing-user", Email: "invitee@example.com"},
			findByEmailErr: nil,
		},
		Logger: testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws1",
		Email:       "invitee@example.com",
		RoleIDs:     []string{"r1"},
		InviterID:   "inviter1",
		Now:         time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrInvitationPayloadInvalid) {
		t.Fatalf("expected ErrInvitationPayloadInvalid, got %v", err)
	}
}
