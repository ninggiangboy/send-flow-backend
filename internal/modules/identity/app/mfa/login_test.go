package mfa

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

func mfaTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type mfaTokenHasherStub struct{}

func (mfaTokenHasherStub) HashToken(string) string { return "hash" }

type mfaTokenGenStub struct{}

func (mfaTokenGenStub) RandomToken(int) (string, error) { return "token", nil }

type mfaIDGenStub struct{}

func (mfaIDGenStub) New() (string, error) { return "id", nil }

type mfaAuthTokenRepoStub struct {
	token *domain.AuthToken
}

func (s *mfaAuthTokenRepoStub) FindByHash(context.Context, string, string) (*domain.AuthToken, error) {
	return s.token, nil
}
func (s *mfaAuthTokenRepoStub) Consume(context.Context, string, time.Time) error { return nil }
func (s *mfaAuthTokenRepoStub) Create(context.Context, domain.AuthToken) error   { return nil }
func (s *mfaAuthTokenRepoStub) DeleteByUserAndPurpose(context.Context, string, string) error {
	return nil
}

type mfaUserWriteStub struct {
	findByID func(context.Context, string) (*domain.User, error)
}

func (s *mfaUserWriteStub) Create(context.Context, domain.User) error { return nil }
func (s *mfaUserWriteStub) UpdatePassword(context.Context, string, string, time.Time) error {
	return nil
}
func (s *mfaUserWriteStub) MarkEmailVerified(context.Context, string, time.Time) error { return nil }
func (s *mfaUserWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}
func (s *mfaUserWriteStub) FindByEmail(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (s *mfaUserWriteStub) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return s.findByID(ctx, id)
}

type mfaTOTPStub struct {
	secret *domain.TOTPSecret
	codes  []domain.RecoveryCode
}

func (s *mfaTOTPStub) UpsertSecret(context.Context, domain.TOTPSecret) error { return nil }
func (s *mfaTOTPStub) FindSecretByUser(context.Context, string) (*domain.TOTPSecret, error) {
	return s.secret, nil
}
func (s *mfaTOTPStub) DeleteSecret(context.Context, string) error { return nil }
func (s *mfaTOTPStub) ReplaceRecoveryCodes(context.Context, string, []domain.RecoveryCode) error {
	return nil
}
func (s *mfaTOTPStub) ListRecoveryCodes(context.Context, string) ([]domain.RecoveryCode, error) {
	return s.codes, nil
}
func (s *mfaTOTPStub) ConsumeRecoveryCode(context.Context, string, time.Time) error { return nil }

type mfaVerifierStub struct{ valid bool }

func (s *mfaVerifierStub) VerifyTOTPCode(string, string, time.Time) bool { return s.valid }

type mfaTokenManagerStub struct{}

func (mfaTokenManagerStub) Issue(_, _ string, now time.Time) (ports.TokenPair, string, string, error) {
	return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "ajti", "rjti", nil
}
func (mfaTokenManagerStub) ParseAccess(string, time.Time) (*ports.AccessClaims, error) {
	return nil, nil
}
func (mfaTokenManagerStub) ParseRefresh(string, time.Time) (*ports.AccessClaims, error) {
	return nil, nil
}

type mfaSessionWriteStub struct {
	create func(context.Context, domain.Session) error
}

func (s *mfaSessionWriteStub) Create(ctx context.Context, session domain.Session) error {
	return s.create(ctx, session)
}
func (mfaSessionWriteStub) RevokeByID(context.Context, string, time.Time) error   { return nil }
func (mfaSessionWriteStub) RevokeByUser(context.Context, string, time.Time) error { return nil }
func (mfaSessionWriteStub) RotateTokens(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}
func (mfaSessionWriteStub) FindByID(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (mfaSessionWriteStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (mfaSessionWriteStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
}

type mfaRefreshStoreStub struct{}

func (mfaRefreshStoreStub) Save(context.Context, string, string, time.Duration) error { return nil }
func (mfaRefreshStoreStub) Find(context.Context, string) (string, error)              { return "", nil }
func (mfaRefreshStoreStub) Delete(context.Context, string) error                      { return nil }
func (mfaRefreshStoreStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

func validMFAToken() *domain.AuthToken {
	return &domain.AuthToken{ID: "tok-1", UserID: "u1", Purpose: domain.AuthTokenPurposeMFAChallenge, ExpiresAt: time.Now().Add(time.Hour)}
}

func TestMFALoginHandlerExecuteTOTPSuccess(t *testing.T) {
	var called bool
	authTokenSvc := shared.NewAuthTokenService(&mfaAuthTokenRepoStub{token: validMFAToken()}, mfaTokenGenStub{}, mfaIDGenStub{}, mfaTokenHasherStub{})
	sessionFactory := shared.NewSessionFactory(mfaIDGenStub{}, mfaTokenManagerStub{}, &mfaSessionWriteStub{
		create: func(context.Context, domain.Session) error { called = true; return nil },
	}, mfaRefreshStoreStub{}, mfaTestLogger())

	h := NewMFALoginHandler(MFALoginOptions{
		AuthTokens: authTokenSvc,
		UsersWrite: &mfaUserWriteStub{findByID: func(context.Context, string) (*domain.User, error) {
			return &domain.User{ID: "u1", Email: "test@example.com"}, nil
		}},
		Totp:           &mfaTOTPStub{secret: &domain.TOTPSecret{Secret: "SECRET"}},
		TokenHasher:    mfaTokenHasherStub{},
		TotpVerifier:   &mfaVerifierStub{valid: true},
		SessionFactory: sessionFactory,
		Logger:         mfaTestLogger(),
	})

	res, err := h.Execute(context.Background(), MFALoginCommand{ChallengeToken: "challenge", Code: "123456", Now: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called || res.User.ID != "u1" {
		t.Fatalf("unexpected result: %+v called=%v", res, called)
	}
}
