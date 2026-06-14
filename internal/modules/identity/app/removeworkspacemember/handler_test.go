package removeworkspacemember

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var testLogger = slog.Default()

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
	return nil, nil
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
	deleteByID func(ctx context.Context, membershipID string) error
}

func (s *membershipsWriteStub) Create(ctx context.Context, membership domain.Membership) error {
	return nil
}
func (s *membershipsWriteStub) DeleteByID(ctx context.Context, membershipID string) error {
	return s.deleteByID(ctx, membershipID)
}
func (s *membershipsWriteStub) UpdateRole(ctx context.Context, membershipID string, role domain.MembershipRole, updatedAt time.Time) error {
	return nil
}
func (s *membershipsWriteStub) UpdateStatus(ctx context.Context, membershipID string, status domain.MembershipStatus, updatedAt time.Time) error {
	return nil
}

func TestRemoveWorkspaceMemberSuccess(t *testing.T) {
	now := time.Now().UTC()
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "remover-mem", WorkspaceID: "ws-1", UserID: "u1", Role: domain.MembershipRoleAdmin}, nil
			},
			findByID: func(_ context.Context, membershipID string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-1", UserID: "u2", Role: domain.MembershipRoleMember}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, membershipID string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 1}, // PermissionWorkspaceManageMembers
				}, nil
			},
		},
		MembershipsWrite: &membershipsWriteStub{
			deleteByID: func(_ context.Context, _ string) error {
				return nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveWorkspaceMember_RemoverNotMember(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1"})
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got: %v", err)
	}
}

func TestRemoveWorkspaceMember_RemoverMembershipReadError(t *testing.T) {
	expectedErr := errors.New("db error")
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, expectedErr
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}

func TestRemoveWorkspaceMember_RemoverRolesReadError(t *testing.T) {
	expectedErr := errors.New("roles error")
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "remover-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return nil, expectedErr
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}

func TestRemoveWorkspaceMember_InsufficientPermissions(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "remover-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-member", Type: domain.RoleTypeMember, PermissionsMask: 0}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1"})
	if !errors.Is(err, domain.ErrMembershipManageDenied) {
		t.Fatalf("expected ErrMembershipManageDenied, got: %v", err)
	}
}

func TestRemoveWorkspaceMember_TargetNotFound(t *testing.T) {
	expectedErr := errors.New("not found")
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "remover-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return nil, expectedErr
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 1}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}

func TestRemoveWorkspaceMember_TargetWrongWorkspace(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "remover-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-2"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 1}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1"})
	if !errors.Is(err, domain.ErrMembershipNotFound) {
		t.Fatalf("expected ErrMembershipNotFound, got: %v", err)
	}
}

func TestRemoveWorkspaceMember_SelfRemoval(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "same-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "same-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 1}}, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "same-mem", RemoverID: "u1"})
	if !errors.Is(err, domain.ErrMembershipManageDenied) {
		t.Fatalf("expected ErrMembershipManageDenied, got: %v", err)
	}
}

func TestRemoveWorkspaceMember_LastOwner(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		MembershipsRead: &membershipsReadStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "remover-mem", WorkspaceID: "ws-1"}, nil
			},
			findByID: func(_ context.Context, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "target-mem", WorkspaceID: "ws-1"}, nil
			},
		},
		RolesRead: &rolesReadStub{
			listByMembership: func(_ context.Context, membershipID string) ([]domain.Role, error) {
				if membershipID == "remover-mem" {
					return []domain.Role{{ID: "role-admin", Type: domain.RoleTypeCustom, PermissionsMask: 1 << 1}}, nil
				}
				return []domain.Role{{ID: "role-owner", Type: domain.RoleTypeOwner}}, nil
			},
			findByType: func(_ context.Context, _ string, _ domain.RoleType) (*domain.Role, error) {
				return &domain.Role{ID: "owner-role"}, nil
			},
			countMembershipsByRole: func(_ context.Context, _, _ string) (int, error) {
				return 1, nil
			},
		},
	})
	err := h.Execute(context.Background(), Command{WorkspaceID: "ws-1", MembershipID: "target-mem", RemoverID: "u1"})
	if !errors.Is(err, domain.ErrLastOwnerCannotBeRemoved) {
		t.Fatalf("expected ErrLastOwnerCannotBeRemoved, got: %v", err)
	}
}
