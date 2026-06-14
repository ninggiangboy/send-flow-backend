package mfalogin

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

type tokenHasherStub struct{}

func (tokenHasherStub) HashToken(token string) string { return "hash" }

type tokenGenStub struct{}

func (tokenGenStub) RandomToken(n int) (string, error) { return "token", nil }

type idGenStub struct{}

func (idGenStub) New() (string, error) { return "id", nil }

type authTokensStub struct {
	token *domain.AuthToken
	err   error
}

func (s *authTokensStub) FindByHash(_ context.Context, purpose, hash string) (*domain.AuthToken, error) {
	return s.token, s.err
}
func (s *authTokensStub) Consume(_ context.Context, tokenID string, at time.Time) error { return nil }
func (s *authTokensStub) Create(_ context.Context, token domain.AuthToken) error        { return nil }
func (s *authTokensStub) DeleteByUserAndPurpose(_ context.Context, userID, purpose string) error {
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

type totpStub struct {
	secret         *domain.TOTPSecret
	findErr        error
	codes          []domain.RecoveryCode
	consumeCodeErr error
	listCodesErr   error
}

func (s *totpStub) UpsertSecret(_ context.Context, sec domain.TOTPSecret) error { return nil }
func (s *totpStub) FindSecretByUser(_ context.Context, userID string) (*domain.TOTPSecret, error) {
	return s.secret, s.findErr
}
func (s *totpStub) DeleteSecret(_ context.Context, userID string) error { return nil }
func (s *totpStub) ReplaceRecoveryCodes(_ context.Context, userID string, codes []domain.RecoveryCode) error {
	return nil
}
func (s *totpStub) ListRecoveryCodes(_ context.Context, userID string) ([]domain.RecoveryCode, error) {
	return s.codes, s.listCodesErr
}
func (s *totpStub) ConsumeRecoveryCode(_ context.Context, codeID string, at time.Time) error {
	return s.consumeCodeErr
}

type totpVerifierStub struct {
	valid bool
}

func (s *totpVerifierStub) VerifyTOTPCode(secret, code string, now time.Time) bool { return s.valid }

func validToken() *domain.AuthToken {
	return &domain.AuthToken{
		ID:        "tok-1",
		UserID:    "u1",
		Purpose:   domain.AuthTokenPurposeMFAChallenge,
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

type sessIdGenStub struct{}

func (sessIdGenStub) New() (string, error) { return "sess-1", nil }

type mfaTokMgrStub struct{}

func (mfaTokMgrStub) Issue(_, _ string, now time.Time) (ports.TokenPair, string, string, error) {
	return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "ajti", "rjti", nil
}
func (mfaTokMgrStub) ParseAccess(string) (*ports.AccessClaims, error)  { return nil, nil }
func (mfaTokMgrStub) ParseRefresh(string) (*ports.AccessClaims, error) { return nil, nil }

type mfaSessWrtStub struct {
	create func(ctx context.Context, session domain.Session) error
}

func (s *mfaSessWrtStub) Create(ctx context.Context, session domain.Session) error {
	return s.create(ctx, session)
}
func (mfaSessWrtStub) RevokeByID(context.Context, string, time.Time) error   { return nil }
func (mfaSessWrtStub) RevokeByUser(context.Context, string, time.Time) error { return nil }
func (mfaSessWrtStub) RotateTokens(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}

type mfaRfrshStub struct{}

func (mfaRfrshStub) Save(context.Context, string, string, time.Duration) error { return nil }
func (mfaRfrshStub) Find(context.Context, string) (string, error)              { return "", nil }
func (mfaRfrshStub) Delete(context.Context, string) error                      { return nil }
func (mfaRfrshStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

func TestExecuteTOTPSuccess(t *testing.T) {
	var sessionCalled bool
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: validToken()}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	sessWrite := &mfaSessWrtStub{
		create: func(_ context.Context, _ domain.Session) error {
			sessionCalled = true
			return nil
		},
	}
	sessionFactory := usecase.NewSessionFactory(&sessIdGenStub{}, mfaTokMgrStub{}, sessWrite, mfaRfrshStub{}, testLogger)
	h := New(Options{
		AuthTokens:     authTokenSvc,
		UsersRead:      &userReadStub{user: &domain.User{ID: "u1", Email: "test@example.com"}},
		Totp:           &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TokenHasher:    tokenHasherStub{},
		TotpVerifier:   &totpVerifierStub{valid: true},
		SessionFactory: sessionFactory,
		Logger:         testLogger,
	})

	res, err := h.Execute(context.Background(), Command{
		ChallengeToken: "challenge-valid",
		Code:           "123456",
		Now:            time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sessionCalled {
		t.Fatal("expected newSession to be called")
	}
	if res.User.ID != "u1" {
		t.Fatalf("expected user u1, got %s", res.User.ID)
	}
}

func TestExecuteRecoveryCodeSuccess(t *testing.T) {
	var sessionCalled bool
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: validToken()}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	sessWrite := &mfaSessWrtStub{
		create: func(_ context.Context, _ domain.Session) error {
			sessionCalled = true
			return nil
		},
	}
	sessionFactory := usecase.NewSessionFactory(&sessIdGenStub{}, mfaTokMgrStub{}, sessWrite, mfaRfrshStub{}, testLogger)
	h := New(Options{
		AuthTokens: authTokenSvc,
		UsersRead:  &userReadStub{user: &domain.User{ID: "u1", Email: "test@example.com"}},
		Totp: &totpStub{
			codes: []domain.RecoveryCode{
				{ID: "rc-1", UserID: "u1", CodeHash: "hash", CreatedAt: time.Now()},
			},
		},
		TokenHasher:    tokenHasherStub{},
		TotpVerifier:   &totpVerifierStub{valid: false},
		SessionFactory: sessionFactory,
		Logger:         testLogger,
	})

	res, err := h.Execute(context.Background(), Command{
		ChallengeToken: "challenge-valid",
		RecoveryCode:   "any-recovery-code",
		Now:            time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sessionCalled {
		t.Fatal("expected newSession to be called")
	}
	if res.User.ID != "u1" {
		t.Fatalf("expected user u1, got %s", res.User.ID)
	}
}

func TestExecuteInvalidChallengeToken(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: nil, err: domain.ErrUnauthorized}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens:  authTokenSvc,
		UsersRead:   &userReadStub{},
		Totp:        &totpStub{},
		TokenHasher: tokenHasherStub{},
		Logger:      testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		ChallengeToken: "invalid",
		Now:            time.Now(),
	})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteChallengeTokenNotFound(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: nil, err: domain.ErrNotFound}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens:  authTokenSvc,
		UsersRead:   &userReadStub{},
		Totp:        &totpStub{},
		TokenHasher: tokenHasherStub{},
		Logger:      testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		ChallengeToken: "missing",
		Now:            time.Now(),
	})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteConsumeAuthTokenErrorPropagated(t *testing.T) {
	upstreamErr := errors.New("unexpected db error")
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: nil, err: upstreamErr}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens:  authTokenSvc,
		UsersRead:   &userReadStub{},
		Totp:        &totpStub{},
		TokenHasher: tokenHasherStub{},
		Logger:      testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		ChallengeToken: "some-token",
		Now:            time.Now(),
	})
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestExecuteUserNotFoundAfterToken(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: validToken()}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens:  authTokenSvc,
		UsersRead:   &userReadStub{err: domain.ErrNotFound},
		Totp:        &totpStub{},
		TokenHasher: tokenHasherStub{},
		Logger:      testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		ChallengeToken: "challenge-valid",
		Code:           "123456",
		Now:            time.Now(),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestExecuteTOTPCodeInvalid(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: validToken()}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens:   authTokenSvc,
		UsersRead:    &userReadStub{user: &domain.User{ID: "u1"}},
		Totp:         &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TokenHasher:  tokenHasherStub{},
		TotpVerifier: &totpVerifierStub{valid: false},
		Logger:       testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		ChallengeToken: "challenge-valid",
		Code:           "wrong",
		Now:            time.Now(),
	})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteTOTPSecretNotFound(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: validToken()}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens:   authTokenSvc,
		UsersRead:    &userReadStub{user: &domain.User{ID: "u1"}},
		Totp:         &totpStub{findErr: errors.New("not found")},
		TokenHasher:  tokenHasherStub{},
		TotpVerifier: &totpVerifierStub{valid: true},
		Logger:       testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		ChallengeToken: "challenge-valid",
		Code:           "123456",
		Now:            time.Now(),
	})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteRecoveryCodeNoMatch(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(&authTokensStub{token: validToken()}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens: authTokenSvc,
		UsersRead:  &userReadStub{user: &domain.User{ID: "u1"}},
		Totp: &totpStub{
			codes: []domain.RecoveryCode{
				{ID: "rc-1", UserID: "u1", CodeHash: "different-hash", CreatedAt: time.Now()},
			},
		},
		TokenHasher:  tokenHasherStub{},
		TotpVerifier: &totpVerifierStub{},
		Logger:       testLogger,
	})

	_, err := h.Execute(context.Background(), Command{
		ChallengeToken: "challenge-valid",
		RecoveryCode:   "wrong-code",
		Now:            time.Now(),
	})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}
