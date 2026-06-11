package getworkspaceaccess

import (
	"context"
	"errors"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

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

func TestExecuteSuccess(t *testing.T) {
	m := &domain.Membership{ID: "m1", WorkspaceID: "ws1", UserID: "u1", Role: domain.MembershipRoleAdmin}
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{membership: m},
	})

	result, err := h.Execute(context.Background(), "ws1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "m1" {
		t.Fatalf("expected membership ID 'm1', got %s", result.ID)
	}
}

func TestExecuteNonMember(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{err: domain.ErrMembershipNotFound},
	})

	_, err := h.Execute(context.Background(), "ws1", "u1")
	if !errors.Is(err, domain.ErrWorkspaceAccessDenied) {
		t.Fatalf("expected ErrWorkspaceAccessDenied, got %v", err)
	}
}

func TestExecuteReadError(t *testing.T) {
	h := New(usecase.Deps{
		MembershipsRead: &membershipsReadStub{err: errors.New("db error")},
	})

	_, err := h.Execute(context.Background(), "ws1", "u1")
	if err == nil || err.Error() != "db error" {
		t.Fatalf("expected 'db error', got %v", err)
	}
}
