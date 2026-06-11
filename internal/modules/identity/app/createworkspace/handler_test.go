package createworkspace

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

type workspacesWriteStub struct {
	err error
}

func (s *workspacesWriteStub) Create(_ context.Context, _ domain.Workspace) error { return s.err }

type membershipsWriteStub struct {
	err error
}

func (s *membershipsWriteStub) Create(_ context.Context, _ domain.Membership) error { return s.err }
func (s *membershipsWriteStub) DeleteByID(_ context.Context, _ string) error        { return nil }
func (s *membershipsWriteStub) UpdateRole(_ context.Context, _ string, _ domain.MembershipRole, _ time.Time) error {
	return nil
}
func (s *membershipsWriteStub) UpdateStatus(_ context.Context, _ string, _ domain.MembershipStatus, _ time.Time) error {
	return nil
}

type rolesWriteStub struct {
	createErr        error
	replaceMemberErr error
}

func (s *rolesWriteStub) Create(_ context.Context, _ domain.Role) error { return s.createErr }
func (s *rolesWriteStub) Update(_ context.Context, _ domain.Role) error { return nil }
func (s *rolesWriteStub) ReplaceMembershipRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return s.replaceMemberErr
}
func (s *rolesWriteStub) ReplaceInvitationRoles(_ context.Context, _ string, _ []string, _ time.Time) error {
	return nil
}

type settingsWriteStub struct{}

func (s *settingsWriteStub) CreateDefault(_ context.Context, _ domain.WorkspaceSettings) error {
	return nil
}
func (s *settingsWriteStub) Upsert(_ context.Context, _ domain.WorkspaceSettings, _ *int64) error {
	return nil
}

func TestExecuteEmptyName(t *testing.T) {
	h := New(usecase.Deps{
		Logger:     testLogger,
		UnitOfWork: noopTx{},
	})

	_, err := h.Execute(context.Background(), Command{Name: "  ", UserID: "u1", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrInvalidWorkspaceName) {
		t.Fatalf("expected ErrInvalidWorkspaceName, got %v", err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	now := time.Now().UTC()
	h := New(usecase.Deps{
		IDGen:            idGenStub{id: "id1"},
		WorkspacesWrite:  &workspacesWriteStub{},
		MembershipsWrite: &membershipsWriteStub{},
		RolesWrite:       &rolesWriteStub{},
		SettingsWrite:    &settingsWriteStub{},
		Logger:           testLogger,
		UnitOfWork:       noopTx{},
	})

	ws, err := h.Execute(context.Background(), Command{Name: "My Workspace", UserID: "u1", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.Name != "My Workspace" {
		t.Fatalf("expected workspace name 'My Workspace', got %s", ws.Name)
	}
	if ws.ID != "id1" {
		t.Fatalf("expected workspace ID 'id1', got %s", ws.ID)
	}
}

type idGenErrStub struct{}

func (idGenErrStub) New() (string, error) { return "", errors.New("idgen error") }

func TestExecuteIDGenError(t *testing.T) {
	h := New(usecase.Deps{
		IDGen:      idGenErrStub{},
		Logger:     testLogger,
		UnitOfWork: noopTx{},
	})

	_, err := h.Execute(context.Background(), Command{Name: "Workspace", UserID: "u1", Now: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecuteWorkspaceNameConflict(t *testing.T) {
	h := New(usecase.Deps{
		IDGen:            idGenStub{id: "id1"},
		WorkspacesWrite:  &workspacesWriteStub{err: domain.ErrWorkspaceNameConflict},
		MembershipsWrite: &membershipsWriteStub{},
		RolesWrite:       &rolesWriteStub{},
		SettingsWrite:    &settingsWriteStub{},
		Logger:           testLogger,
		UnitOfWork:       noopTx{},
	})

	_, err := h.Execute(context.Background(), Command{Name: "My Workspace", UserID: "u1", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrWorkspaceNameConflict) {
		t.Fatalf("expected ErrWorkspaceNameConflict, got %v", err)
	}
}
