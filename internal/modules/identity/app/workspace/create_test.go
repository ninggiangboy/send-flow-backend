package workspace

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

func workspaceTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type workspaceIDGenStub struct {
	ids []string
	idx int
}

func (s *workspaceIDGenStub) New() (string, error) {
	if s.idx >= len(s.ids) {
		return "", errors.New("no ids left")
	}
	id := s.ids[s.idx]
	s.idx++
	return id, nil
}

type workspaceWriteStub struct {
	create func(context.Context, domain.Workspace) error
}

func (s *workspaceWriteStub) Create(ctx context.Context, ws domain.Workspace) error {
	return s.create(ctx, ws)
}
func (s *workspaceWriteStub) FindByID(context.Context, string) (*domain.Workspace, error) {
	return nil, nil
}
func (s *workspaceWriteStub) ListByUser(context.Context, string) ([]domain.Workspace, error) {
	return nil, nil
}

type membershipWriteStub struct {
	create func(context.Context, domain.Membership) error
}

func (s *membershipWriteStub) Create(ctx context.Context, membership domain.Membership) error {
	return s.create(ctx, membership)
}
func (s *membershipWriteStub) FindByID(context.Context, string) (*domain.Membership, error) {
	return nil, domain.ErrMembershipNotFound
}
func (s *membershipWriteStub) FindByWorkspaceAndUser(context.Context, string, string) (*domain.Membership, error) {
	return nil, domain.ErrMembershipNotFound
}
func (s *membershipWriteStub) ListByWorkspace(context.Context, string) ([]domain.Membership, error) {
	return nil, nil
}
func (s *membershipWriteStub) CountByWorkspaceAndRole(context.Context, string, domain.MembershipRole) (int, error) {
	return 0, nil
}
func (s *membershipWriteStub) DeleteByID(context.Context, string) error { return nil }
func (s *membershipWriteStub) UpdateRole(context.Context, string, domain.MembershipRole, time.Time) error {
	return nil
}
func (s *membershipWriteStub) UpdateStatus(context.Context, string, domain.MembershipStatus, time.Time) error {
	return nil
}

type rolesWriteStub struct{}

func (rolesWriteStub) FindByID(context.Context, string, string) (*domain.Role, error) {
	return nil, domain.ErrRoleNotFound
}
func (rolesWriteStub) FindByType(context.Context, string, domain.RoleType) (*domain.Role, error) {
	return nil, domain.ErrRoleNotFound
}
func (rolesWriteStub) FindByIDs(context.Context, string, []string) ([]domain.Role, error) {
	return nil, nil
}
func (rolesWriteStub) ListByWorkspace(context.Context, string) ([]domain.Role, error) {
	return nil, nil
}
func (rolesWriteStub) ListByMembership(context.Context, string) ([]domain.Role, error) {
	return nil, nil
}
func (rolesWriteStub) ListByInvitation(context.Context, string) ([]domain.Role, error) {
	return nil, nil
}
func (rolesWriteStub) CountMembershipsByRole(context.Context, string, string) (int, error) {
	return 0, nil
}
func (rolesWriteStub) Create(context.Context, domain.Role) error { return nil }
func (rolesWriteStub) Update(context.Context, domain.Role) error { return nil }
func (rolesWriteStub) ReplaceMembershipRoles(context.Context, string, []string, time.Time) error {
	return nil
}
func (rolesWriteStub) ReplaceInvitationRoles(context.Context, string, []string, time.Time) error {
	return nil
}

type settingsWriteStub struct{}

func (settingsWriteStub) GetByWorkspace(context.Context, string) (*domain.WorkspaceSettings, error) {
	return nil, nil
}
func (settingsWriteStub) CreateDefault(context.Context, domain.WorkspaceSettings) error  { return nil }
func (settingsWriteStub) Upsert(context.Context, domain.WorkspaceSettings, *int64) error { return nil }

type workspaceUnitOfWorkStub struct{}

func (workspaceUnitOfWorkStub) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestCreateHandlerExecuteCreatesWorkspace(t *testing.T) {
	var created bool
	h := NewCreateHandler(CreateOptions{
		IdGen:            &workspaceIDGenStub{ids: []string{"ws_1", "membership_1", "role_1", "role_2", "role_3"}},
		WorkspacesWrite:  &workspaceWriteStub{create: func(context.Context, domain.Workspace) error { created = true; return nil }},
		MembershipsWrite: &membershipWriteStub{create: func(context.Context, domain.Membership) error { return nil }},
		RolesWrite:       rolesWriteStub{},
		SettingsWrite:    settingsWriteStub{},
		UnitOfWork:       workspaceUnitOfWorkStub{},
		Logger:           workspaceTestLogger(),
	})

	ws, err := h.Execute(context.Background(), CreateCommand{Name: "My Workspace", UserID: "user_1", Now: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created || ws.ID != "ws_1" {
		t.Fatalf("unexpected workspace: %+v created=%v", ws, created)
	}
}
