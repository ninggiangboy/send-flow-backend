package oauthexchange

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

var testLogger = slog.Default()

type providerStub struct {
	identity *domain.OAuthIdentity
	err      error
}

func (s *providerStub) Name() string                               { return "google" }
func (s *providerStub) Type() string                               { return "oauth2" }
func (s *providerStub) DisplayName() string                        { return "Google" }
func (s *providerStub) Enabled() bool                              { return true }
func (s *providerStub) BuildAuthURL(string, string, string) string { return "" }
func (s *providerStub) Exchange(context.Context, string, string, string) (*domain.OAuthIdentity, error) {
	return s.identity, s.err
}

type oauthStateStoreStub struct {
	state *ports.OAuthState
	err   error
}

func (s *oauthStateStoreStub) Save(context.Context, string, ports.OAuthState, time.Duration) error {
	return nil
}
func (s *oauthStateStoreStub) GetAndDelete(context.Context, string) (*ports.OAuthState, error) {
	return s.state, s.err
}

type externalReadStub struct {
	acc *domain.ExternalAuthAccount
	err error
}

func (s *externalReadStub) FindByProviderIdentity(context.Context, string, string) (*domain.ExternalAuthAccount, error) {
	return s.acc, s.err
}

type externalWriteStub struct{ touched bool }

func (s *externalWriteStub) Create(context.Context, domain.ExternalAuthAccount) error { return nil }
func (s *externalWriteStub) TouchLogin(context.Context, string, time.Time) error {
	s.touched = true
	return nil
}

type userReadStub struct {
	user *domain.User
	err  error
}

func (s *userReadStub) FindByEmail(context.Context, string) (*domain.User, error) {
	return s.user, s.err
}
func (s *userReadStub) FindByID(context.Context, string) (*domain.User, error) { return s.user, s.err }

type userWriteStub struct{}

func (s *userWriteStub) Create(context.Context, domain.User) error { return nil }
func (s *userWriteStub) UpdatePassword(context.Context, string, string, time.Time) error {
	return nil
}
func (s *userWriteStub) MarkEmailVerified(context.Context, string, time.Time) error { return nil }
func (s *userWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}

func TestExecuteInvalidState(t *testing.T) {
	h := New(usecase.Deps{
		Providers:  map[string]ports.OAuthProvider{"google": &providerStub{}},
		OAuthState: &oauthStateStoreStub{state: nil},
		Logger:     testLogger,
	}, func(context.Context, usecase.NewSessionInput) (*usecase.SessionContext, error) {
		return nil, nil
	})
	_, _, err := h.Execute(context.Background(), Command{Provider: "google", State: "s1", RedirectURI: "http://localhost/cb"})
	if !errors.Is(err, domain.ErrInvalidOAuthState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func TestExecuteSuccessWithExistingAccount(t *testing.T) {
	h := New(usecase.Deps{
		Providers: map[string]ports.OAuthProvider{
			"google": &providerStub{identity: &domain.OAuthIdentity{ProviderUserID: "pid-1", Email: "a@example.com"}},
		},
		OAuthState:     &oauthStateStoreStub{state: &ports.OAuthState{Provider: "google", RedirectURI: "http://localhost/cb", CodeVerifier: "v"}},
		ExternalsRead:  &externalReadStub{acc: &domain.ExternalAuthAccount{ID: "acc1", UserID: "u1"}},
		ExternalsWrite: &externalWriteStub{},
		UsersRead:      &userReadStub{user: &domain.User{ID: "u1", Email: "a@example.com"}},
		UsersWrite:     &userWriteStub{},
		Logger:         testLogger,
	}, func(_ context.Context, in usecase.NewSessionInput) (*usecase.SessionContext, error) {
		return &usecase.SessionContext{User: in.User}, nil
	})

	out, identity, err := h.Execute(context.Background(), Command{
		Provider:     "google",
		Code:         "code",
		State:        "s1",
		RedirectURI:  "http://localhost/cb",
		CodeVerifier: "v",
		Now:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != "u1" || identity.ProviderUserID != "pid-1" {
		t.Fatalf("unexpected output: %+v %+v", out, identity)
	}
}
