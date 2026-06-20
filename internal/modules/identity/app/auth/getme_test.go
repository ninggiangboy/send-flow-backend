package auth

import (
	"context"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

func TestGetMeHandlerExecuteReturnsUser(t *testing.T) {
	h := NewGetMeHandler(&authUserReadStub{
		findByID: func(context.Context, string) (*domain.User, error) {
			return &domain.User{ID: "user_1", Email: "user@example.com"}, nil
		},
	}, testLogger())

	user, err := h.Execute(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user_1" {
		t.Fatalf("expected user_1, got %+v", user)
	}
}
