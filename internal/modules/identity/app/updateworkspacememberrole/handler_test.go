package updateworkspacememberrole

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var testLogger = slog.Default()

type noopTx struct{}

func (t *noopTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type membershipsReadStub struct {
	findByWorkspaceAndUser func(ctx context.Context, workspaceID, userID string) (*domain.Membership, error)
	findByID               func(ctx context.Context, membershipID string) (*domain.Membership, error)
}

func (s *membershipsReadStub) FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	return s.findByWorkspaceAndUser(ctx, workspaceID, userID)
}
func (s *membershipsReadStub) FindByID(ctx context.Context, membershipID string) (*domain.Membership, error) {
	return s.findByID(ctx, membershipID)
}
func (s *membershipsReadStub) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Membership, error) {
	return nil, nil
}
func (s *membershipsReadStub) CountByWorkspaceAndRole(ctx context.Context, workspaceID string, role domain.MembershipRole) (int, error) {
	return 0, nil
}

type rolesReadStub struct {
	listByMembership       func(ctx context.Context, membershipID string) ([]domain.Role, error)
	findByIDs              func(ctx context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error)
	findByType             func(ctx context.Context, workspaceID string, roleType domain.RoleType) (*domain.Role, error)
	countMembershipsByRole func(ctx context.Context, workspaceID, roleID string) (int, error)
}

func (s *rolesReadStub) FindByID(ctx context.Context, workspaceID, roleID string) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) FindByType(ctx context.Context, workspaceID string, roleType domain.RoleType) (*domain.Role, error) {
	return s.findByType(ctx, workspaceID, roleType)
}
func (s *rolesReadStub) FindByIDs(ctx context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error) {
	return s.findByIDs(ctx, workspaceID, roleIDs)
}
func (s *rolesReadStub) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) ListByMembership(ctx context.Context, membershipID string) ([]domain.Role, error) {
	return s.listByMembership(ctx, membershipID)
}
func (s *rolesReadStub) ListByInvitation(ctx context.Context, invitationID string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesReadStub) CountMembershipsByRole(ctx context.Context, workspaceID, roleID string) (int, error) {
	return s.countMembershipsByRole(ctx, workspaceID, roleID)
}

type membershipsWriteStub struct {
	updateRole func(ctx context.Context, membershipID string, role domain.MembershipRole, updatedAt time.Time) error
}

func (s *membershipsWriteStub) Create(ctx context.Context, membership domain.Membership) error {
	return nil
}
func (s *membershipsWriteStub) DeleteByID(ctx context.Context, membershipID string) error {
	return nil
}
func (s *membershipsWriteStub) UpdateRole(ctx context.Context, membershipID string, role domain.MembershipRole, updatedAt time.Time) error {
	return s.updateRole(ctx, membershipID, role, updatedAt)
}
func (s *membershipsWriteStub) UpdateStatus(ctx context.Context, membershipID string, status domain.MembershipStatus, updatedAt time.Time) error {
	return nil
}

type rolesWriteStub struct {
	replaceMembershipRoles func(ctx context.Context, membershipID string, roleIDs []string, updatedAt time.Time) error
}

func (s *rolesWriteStub) Create(ctx context.Context, role domain.Role) error {
	return nil
}
func (s *rolesWriteStub) Update(ctx context.Context, role domain.Role) error {
	return nil
}
func (s *rolesWriteStub) ReplaceMembershipRoles(ctx context.Context, membershipID string, roleIDs []string, updatedAt time.Time) error {
	return s.replaceMembershipRoles(ctx, membershipID, roleIDs, updatedAt)
}
func (s *rolesWriteStub) ReplaceInvitationRoles(ctx context.Context, invitationID string, roleIDs []string, updatedAt time.Time) error {
	return nil
}

func TestUpdateWorkspaceMemberRoleSuccess(t *testing.T) {
	now := time.Now().UTC()
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, membershipID string) ([]domain.Role, error) {
				if membershipID == "updater-mem" {
					return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 2}}, nil
				}
				return []domain.Role{{ID: "role-member", Type: domain.RoleTypeMember}}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-member", Type: domain.RoleTypeMember, Status: domain.RoleStatusActive}}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			replaceMembershipRoles: func(_ context.Context, _ string, _ []string, _ time.Time) error {
				return nil
			},
		},
		MembershipsWrite: &membershipsWriteStub{
			updateRole: func(_ context.Context, _ string, _ domain.MembershipRole, _ time.Time) error {
				return nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{"role-member"}, Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateWorkspaceMemberRole_UpdaterNotMember(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1"})
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got: %v", err)
	}
}

func TestUpdateWorkspaceMemberRole_UpdaterMembershipReadError(t *testing.T) {
	expectedErr := errors.New("db error")
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, expectedErr
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}

func TestUpdateWorkspaceMemberRole_InsufficientPermissions(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-member", Type: domain.RoleTypeMember, PermissionsMask: 0}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{"role-member"}})
	if !errors.Is(err, domain.ErrMembershipManageDenied) {
		t.Fatalf("expected ErrMembershipManageDenied, got: %v", err)
	}
}

func TestUpdateWorkspaceMemberRole_TargetNotFound(t *testing.T) {
	expectedErr := errors.New("not found")
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return nil, expectedErr
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 2}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{"role-member"}})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}

func TestUpdateWorkspaceMemberRole_TargetWrongWorkspace(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-2"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 2}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{"role-member"}})
	if !errors.Is(err, domain.ErrMembershipNotFound) {
		t.Fatalf("expected ErrMembershipNotFound, got: %v", err)
	}
}

func TestUpdateWorkspaceMemberRole_EmptyRoleIDs(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 2}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{}})
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got: %v", err)
	}
}

func TestUpdateWorkspaceMemberRole_RoleNotFound(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 2}}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{"nonexistent"}})
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got: %v", err)
	}
}

func TestUpdateWorkspaceMemberRole_RoleNotActive(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 2}}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{{ID: "archived-role", Type: domain.RoleTypeCustom, Status: domain.RoleStatusArchived}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{"archived-role"}})
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got: %v", err)
	}
}

func TestUpdateWorkspaceMemberRole_DemoteLastOwner(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: &noopTx{},
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "updater-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, membershipID string) ([]domain.Role, error) {
				if membershipID == "updater-mem" {
					return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 2}}, nil
				}
				return []domain.Role{{ID: "role-owner", Type: domain.RoleTypeOwner}}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-member", Type: domain.RoleTypeMember, Status: domain.RoleStatusActive}}, nil
			},
			findByType: func(_ context.Context, _ string, _ domain.RoleType) (*domain.Role, error) {
				return &domain.Role{ID: "owner-role"}, nil
			},
			countMembershipsByRole: func(_ context.Context, _, _ string) (int, error) {
				return 1, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", UpdaterID: "u1", RoleIDs: []string{"role-member"}, Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrLastOwnerCannotBeRemoved) {
		t.Fatalf("expected ErrLastOwnerCannotBeRemoved, got: %v", err)
	}
}
