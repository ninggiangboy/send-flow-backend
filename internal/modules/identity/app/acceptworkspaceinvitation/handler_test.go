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

type userWriteStub struct {
	findByID func(ctx context.Context, userID string) (*domain.User, error)
}

func (s *userWriteStub) Create(context.Context, domain.User) error                       { return nil }
func (s *userWriteStub) UpdatePassword(context.Context, string, string, time.Time) error { return nil }
func (s *userWriteStub) MarkEmailVerified(context.Context, string, time.Time) error      { return nil }
func (s *userWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}
func (s *userWriteStub) FindByEmail(context.Context, string) (*domain.User, error) { return nil, nil }
func (s *userWriteStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	if s.findByID != nil {
		return s.findByID(ctx, userID)
	}
	return nil, nil
}

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
	listByInvitation func(ctx context.Context, invitationID string) ([]domain.Role, error)
}

func (s *rolesWriteStub) Create(_ context.Context, _ domain.Role) error { return nil }
func (s *rolesWriteStub) Update(_ context.Context, _ domain.Role) error { return nil }
func (s *rolesWriteStub) ReplaceMembershipRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return nil
}
func (s *rolesWriteStub) ReplaceInvitationRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return nil
}
func (s *rolesWriteStub) FindByID(_ context.Context, _, _ string) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) FindByType(_ context.Context, _ string, _ domain.RoleType) (*domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) FindByIDs(_ context.Context, _ string, _ []string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) ListByMembership(_ context.Context, _ string) ([]domain.Role, error) {
	return nil, nil
}
func (s *rolesWriteStub) ListByInvitation(ctx context.Context, invitationID string) ([]domain.Role, error) {
	if s.listByInvitation != nil {
		return s.listByInvitation(ctx, invitationID)
	}
	return nil, nil
}
func (s *rolesWriteStub) CountMembershipsByRole(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

type invitationsWriteStub struct {
	findByToken func(ctx context.Context, token string) (*domain.Invitation, error)
}

func (s *invitationsWriteStub) Create(_ context.Context, _ domain.Invitation) error { return nil }
func (s *invitationsWriteStub) UpdateStatus(_ context.Context, _ string, _ domain.InvitationStatus, _ time.Time) error {
	return nil
}
func (s *invitationsWriteStub) FindByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	if s.findByToken != nil {
		return s.findByToken(ctx, token)
	}
	return nil, nil
}
func (s *invitationsWriteStub) ListByWorkspace(_ context.Context, _ string) ([]domain.Invitation, error) {
	return nil, nil
}

func TestExecuteSuccess(t *testing.T) {
	now := time.Now().UTC()
	invitation := domain.NewInvitation("inv1", "ws1", "user@example.com", "token123", domain.MembershipRoleMember, now.Add(24*time.Hour), now)
	h := New(Options{
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return &invitation, nil
			},
		},
		UsersWrite: &userWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "u1", Email: "user@example.com"}, nil
			},
		},
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
			},
		},
		RolesWrite: &rolesWriteStub{
			listByInvitation: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Name: "Member", Type: domain.RoleTypeMember, Status: domain.RoleStatusActive},
				}, nil
			},
		},
		IdGen:      idGenStub{id: "m1"},
		UnitOfWork: noopTx{},
		Logger:     testLogger,
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
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return nil, domain.ErrInvitationNotFound
			},
		},
		Logger: testLogger,
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
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return &invitation, nil
			},
		},
		Logger: testLogger,
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
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return &invitation, nil
			},
		},
		Logger: testLogger,
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
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return &invitation, nil
			},
		},
		UsersWrite: &userWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "u1", Email: "user@example.com"}, nil
			},
		},
		Logger: testLogger,
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
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return &invitation, nil
			},
		},
		UsersWrite: &userWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "u1", Email: "user@example.com"}, nil
			},
		},
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "u1"}, nil
			},
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
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return &invitation, nil
			},
		},
		UsersWrite: &userWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "u1", Email: "user@example.com"}, nil
			},
		},
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
			},
		},
		RolesWrite: &rolesWriteStub{
			listByInvitation: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{}, nil
			},
		},
		Logger: testLogger,
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
		InvitationsWrite: &invitationsWriteStub{
			findByToken: func(_ context.Context, _ string) (*domain.Invitation, error) {
				return &invitation, nil
			},
		},
		UsersWrite: &userWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "u1", Email: "user@example.com"}, nil
			},
		},
		MembershipsWrite: &membershipsWriteStub{
			findByWorkspaceAndUser: func(_ context.Context, _, _ string) (*domain.Membership, error) {
				return nil, domain.ErrMembershipNotFound
			},
		},
		RolesWrite: &rolesWriteStub{
			listByInvitation: func(_ context.Context, _ string) ([]domain.Role, error) {
				return []domain.Role{
					{ID: "r1", Name: "Member", Type: domain.RoleTypeMember, Status: domain.RoleStatusActive},
				}, nil
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
