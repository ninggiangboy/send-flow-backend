package inviteworkspacemember

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
	id string
}

func (s idGenStub) New() (string, error) { return s.id, nil }

type membershipsWriteStub struct {
	findByWorkspaceAndUser func(ctx context.Context, workspaceID, userID string) (*domain.Membership, error)
}

func (s *membershipsWriteStub) Create(_ context.Context, _ domain.Membership) error { return nil }
func (s *membershipsWriteStub) DeleteByID(_ context.Context, _ string) error        { return nil }
func (s *membershipsWriteStub) UpdateRole(_ context.Context, _ string, _ domain.MembershipRole, _ time.Time) error {
	return nil
}
func (s *membershipsWriteStub) UpdateStatus(_ context.Context, _ string, _ domain.MembershipStatus, _ time.Time) error {
	return nil
}
func (s *membershipsWriteStub) FindByID(_ context.Context, _ string) (*domain.Membership, error) {
	return nil, nil
}
func (s *membershipsWriteStub) FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	if s.findByWorkspaceAndUser != nil {
		return s.findByWorkspaceAndUser(ctx, workspaceID, userID)
	}
	return nil, nil
}
func (s *membershipsWriteStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Membership, error) {
	return nil, nil
}
func (s *membershipsWriteStub) CountByWorkspaceAndRole(_ context.Context, _ string, _ domain.MembershipRole) (int, error) {
	return 0, nil
}

type rolesWriteStub struct {
	createErr        error
	replaceMemberErr error
	replaceInviteErr error
	listByMembership func(ctx context.Context, membershipID string) ([]domain.Role, error)
	findByIDs        func(ctx context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error)
}

func (s *rolesWriteStub) Create(_ context.Context, _ domain.Role) error { return s.createErr }
func (s *rolesWriteStub) Update(_ context.Context, _ domain.Role) error { return nil }
func (s *rolesWriteStub) ReplaceMembershipRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return s.replaceMemberErr
}
func (s *rolesWriteStub) ReplaceInvitationRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return s.replaceInviteErr
}
func (s *rolesWriteStub) FindByID(_ context.Context, _, _ string) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) FindByType(_ context.Context, _ string, _ domain.RoleType) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) FindByIDs(ctx context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error) {
	if s.findByIDs != nil {
		return s.findByIDs(ctx, workspaceID, roleIDs)
	}
	return nil, nil
}
func (s *rolesWriteStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) ListByMembership(ctx context.Context, membershipID string) ([]domain.Role, error) {
	if s.listByMembership != nil {
		return s.listByMembership(ctx, membershipID)
	}
	return nil, nil
}
func (s *rolesWriteStub) ListByInvitation(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) CountMembershipsByRole(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

type userWriteStub struct {
	findByID    func(ctx context.Context, userID string) (*domain.User, error)
	findByEmail func(ctx context.Context, email string) (*domain.User, error)
}

func (s *userWriteStub) Create(context.Context, domain.User) error                       { return nil }
func (s *userWriteStub) UpdatePassword(context.Context, string, string, time.Time) error { return nil }
func (s *userWriteStub) MarkEmailVerified(context.Context, string, time.Time) error      { return nil }
func (s *userWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}
func (s *userWriteStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	if s.findByID != nil {
		return s.findByID(ctx, userID)
	}
	return nil, nil
}
func (s *userWriteStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if s.findByEmail != nil {
		return s.findByEmail(ctx, email)
	}
	return nil, nil
}

type invitationsWriteStub struct{}

func (s *invitationsWriteStub) Create(_ context.Context, _ domain.Invitation) error { return nil }
func (s *invitationsWriteStub) UpdateStatus(_ context.Context, _ string, _ domain.InvitationStatus, _ time.Time) error {
	return nil
}
func (s *invitationsWriteStub) FindByToken(_ context.Context, _ string) (*domain.Invitation, error) {
	return nil, nil
}
func (s *invitationsWriteStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Invitation, error) {
	return nil, nil
}

func TestExecuteSuccess(t *testing.T) {
	now := time.Now().UTC()
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
			},
		},
		UsersWrite: &userWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "inviter1", Email: "inviter@example.com"}, nil
			},
			findByEmail: func(_ context.Context, _ string) (*domain.User, error) {
				return nil, domain.ErrNotFound
			},
		},
		InvitationsWrite: &invitationsWriteStub{},
		IdGen:            idGenStub{id: "id1"},
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
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
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
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got %v", err)
	}
}

func TestExecuteInviterLacksPermission(t *testing.T) {
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeMember, PermissionsMask: 0, Status: domain.RoleStatusActive},
				}, nil
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
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
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
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{}, nil
			},
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
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r2", Type: domain.RoleTypeMember, PermissionsMask: 0, Status: domain.RoleStatusArchived},
				}, nil
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
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
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
	h := New(Options{
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "inviter1"}, nil
			},
		},
		RolesWrite: &rolesWriteStub{
			listByMembership: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
			},
			findByIDs: func(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Type: domain.RoleTypeOwner, PermissionsMask: domain.AllPermissionsMask(), Status: domain.RoleStatusActive},
				}, nil
			},
		},
		UsersWrite: &userWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "inviter1", Email: "inviter@example.com"}, nil
			},
			findByEmail: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "existing-user", Email: "invitee@example.com"}, nil
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
	if !errors.Is(err, domain.ErrInvitationPayloadInvalid) {
		t.Fatalf("expected ErrInvitationPayloadInvalid, got %v", err)
	}
}
