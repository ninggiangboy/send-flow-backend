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

type noopTx struct{}

func (noopTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type oauthIdGenStub struct{}

func (oauthIdGenStub) New() (string, error) { return "id1", nil }

type oauthTokMgrStub struct{}

func (oauthTokMgrStub) Issue(_, _ string, now time.Time) (ports.TokenPair, string, string, error) {
	return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "ajti", "rjti", nil
}
func (oauthTokMgrStub) ParseAccess(string, time.Time) (*ports.AccessClaims, error)  { return nil, nil }
func (oauthTokMgrStub) ParseRefresh(string, time.Time) (*ports.AccessClaims, error) { return nil, nil }

type oauthSessWrtStub struct{}

func (oauthSessWrtStub) Create(context.Context, domain.Session) error          { return nil }
func (oauthSessWrtStub) RevokeByID(context.Context, string, time.Time) error   { return nil }
func (oauthSessWrtStub) RevokeByUser(context.Context, string, time.Time) error { return nil }
func (oauthSessWrtStub) RotateTokens(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}

type oauthRfrshStub struct{}

func (oauthRfrshStub) Save(context.Context, string, string, time.Duration) error { return nil }
func (oauthRfrshStub) Find(context.Context, string) (string, error)              { return "", nil }
func (oauthRfrshStub) Delete(context.Context, string) error                      { return nil }
func (oauthRfrshStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

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
	h := New(Options{
		Providers:  map[string]ports.OAuthProvider{"google": &providerStub{}},
		OauthState: &oauthStateStoreStub{state: nil},
		Logger:     testLogger,
	})
	_, _, err := h.Execute(context.Background(), Command{Provider: "google", State: "s1", RedirectURI: "http://localhost/cb"})
	if !errors.Is(err, domain.ErrInvalidOAuthState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func TestExecuteSuccessWithExistingAccount(t *testing.T) {
	sessionFactory := usecase.NewSessionFactory(oauthIdGenStub{}, oauthTokMgrStub{}, oauthSessWrtStub{}, oauthRfrshStub{}, testLogger)
	h := New(Options{
		Providers: map[string]ports.OAuthProvider{
			"google": &providerStub{identity: &domain.OAuthIdentity{ProviderUserID: "pid-1", Email: "a@example.com"}},
		},
		OauthState:     &oauthStateStoreStub{state: &ports.OAuthState{Provider: "google", RedirectURI: "http://localhost/cb", CodeVerifier: "v"}},
		ExternalsRead:  &externalReadStub{acc: &domain.ExternalAuthAccount{ID: "acc1", UserID: "u1"}},
		ExternalsWrite: &externalWriteStub{},
		UsersRead:      &userReadStub{user: &domain.User{ID: "u1", Email: "a@example.com"}},
		UsersWrite:     &userWriteStub{},
		IdGen:          oauthIdGenStub{},
		UnitOfWork:     noopTx{},
		SessionFactory: sessionFactory,
		Logger:         testLogger,
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
