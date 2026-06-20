package auth

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type authUserWriteStub struct {
	findByEmail       func(ctx context.Context, email string) (*domain.User, error)
	findByID          func(ctx context.Context, userID string) (*domain.User, error)
	create            func(ctx context.Context, user domain.User) error
	updatePassword    func(ctx context.Context, userID, hash string, changedAt time.Time) error
	markEmailVerified func(ctx context.Context, userID string, at time.Time) error
	setMFAEnabledAt   func(ctx context.Context, userID string, enabledAt *time.Time, updatedAt time.Time) error
}

func (s *authUserWriteStub) Create(ctx context.Context, user domain.User) error {
	if s.create != nil {
		return s.create(ctx, user)
	}
	return nil
}

func (s *authUserWriteStub) UpdatePassword(ctx context.Context, userID, hash string, changedAt time.Time) error {
	if s.updatePassword != nil {
		return s.updatePassword(ctx, userID, hash, changedAt)
	}
	return nil
}

func (s *authUserWriteStub) MarkEmailVerified(ctx context.Context, userID string, at time.Time) error {
	if s.markEmailVerified != nil {
		return s.markEmailVerified(ctx, userID, at)
	}
	return nil
}

func (s *authUserWriteStub) SetMFAEnabledAt(ctx context.Context, userID string, enabledAt *time.Time, updatedAt time.Time) error {
	if s.setMFAEnabledAt != nil {
		return s.setMFAEnabledAt(ctx, userID, enabledAt, updatedAt)
	}
	return nil
}

func (s *authUserWriteStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if s.findByEmail != nil {
		return s.findByEmail(ctx, email)
	}
	return nil, domain.ErrNotFound
}

func (s *authUserWriteStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	if s.findByID != nil {
		return s.findByID(ctx, userID)
	}
	return nil, domain.ErrNotFound
}

type authUserReadStub struct {
	findByEmail func(ctx context.Context, email string) (*domain.User, error)
	findByID    func(ctx context.Context, userID string) (*domain.User, error)
}

func (s *authUserReadStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if s.findByEmail != nil {
		return s.findByEmail(ctx, email)
	}
	return nil, domain.ErrNotFound
}

func (s *authUserReadStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	if s.findByID != nil {
		return s.findByID(ctx, userID)
	}
	return nil, domain.ErrNotFound
}

type authHasherStub struct {
	hash    string
	hashErr error
	cmpErr  error
}

func (s *authHasherStub) Hash(string) (string, error)  { return s.hash, s.hashErr }
func (s *authHasherStub) Compare(string, string) error { return s.cmpErr }

type authPasswordValidatorStub struct {
	err error
}

func (s authPasswordValidatorStub) Validate(string) error { return s.err }

type authIDGenStub struct {
	id  string
	err error
}

func (s authIDGenStub) New() (string, error) { return s.id, s.err }

type authTokenManagerStub struct {
	issue        func(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error)
	parseAccess  func(token string, now time.Time) (*ports.AccessClaims, error)
	parseRefresh func(token string, now time.Time) (*ports.AccessClaims, error)
}

func (s *authTokenManagerStub) Issue(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error) {
	return s.issue(userID, sessionID, now)
}

func (s *authTokenManagerStub) ParseAccess(token string, now time.Time) (*ports.AccessClaims, error) {
	return s.parseAccess(token, now)
}

func (s *authTokenManagerStub) ParseRefresh(token string, now time.Time) (*ports.AccessClaims, error) {
	return s.parseRefresh(token, now)
}

type authSessionWriteStub struct {
	create       func(ctx context.Context, session domain.Session) error
	revokeByUser func(ctx context.Context, userID string, at time.Time) error
	rotateTokens func(ctx context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, now time.Time) error
	findByID     func(ctx context.Context, sessionID string) (*domain.Session, error)
}

func (s *authSessionWriteStub) Create(ctx context.Context, session domain.Session) error {
	if s.create != nil {
		return s.create(ctx, session)
	}
	return nil
}

func (s *authSessionWriteStub) RevokeByID(context.Context, string, time.Time) error { return nil }

func (s *authSessionWriteStub) RevokeByUser(ctx context.Context, userID string, at time.Time) error {
	if s.revokeByUser != nil {
		return s.revokeByUser(ctx, userID, at)
	}
	return nil
}

func (s *authSessionWriteStub) RotateTokens(ctx context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, now time.Time) error {
	if s.rotateTokens != nil {
		return s.rotateTokens(ctx, sessionID, accessJTI, refreshJTI, expiresAt, now)
	}
	return nil
}

func (s *authSessionWriteStub) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	if s.findByID != nil {
		return s.findByID(ctx, sessionID)
	}
	return nil, domain.ErrNotFound
}

func (s *authSessionWriteStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, domain.ErrNotFound
}

func (s *authSessionWriteStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
}

type authSessionReadStub struct {
	findByID   func(ctx context.Context, sessionID string) (*domain.Session, error)
	listByUser func(ctx context.Context, userID string, now time.Time) ([]domain.Session, error)
}

func (s *authSessionReadStub) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	if s.findByID != nil {
		return s.findByID(ctx, sessionID)
	}
	return nil, domain.ErrNotFound
}

func (s *authSessionReadStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, domain.ErrNotFound
}

func (s *authSessionReadStub) ListByUser(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	if s.listByUser != nil {
		return s.listByUser(ctx, userID, now)
	}
	return nil, nil
}

type authRefreshStoreStub struct {
	save    func(ctx context.Context, refreshJTI, sessionID string, ttl time.Duration) error
	find    func(ctx context.Context, refreshJTI string) (string, error)
	delete  func(ctx context.Context, refreshJTI string) error
	replace func(ctx context.Context, oldJTI, newJTI, sessionID string, ttl time.Duration) error
}

func (s *authRefreshStoreStub) Save(ctx context.Context, refreshJTI, sessionID string, ttl time.Duration) error {
	if s.save != nil {
		return s.save(ctx, refreshJTI, sessionID, ttl)
	}
	return nil
}

func (s *authRefreshStoreStub) Find(ctx context.Context, refreshJTI string) (string, error) {
	if s.find != nil {
		return s.find(ctx, refreshJTI)
	}
	return "", ports.ErrCacheMiss
}

func (s *authRefreshStoreStub) Delete(ctx context.Context, refreshJTI string) error {
	if s.delete != nil {
		return s.delete(ctx, refreshJTI)
	}
	return nil
}

func (s *authRefreshStoreStub) Replace(ctx context.Context, oldJTI, newJTI, sessionID string, ttl time.Duration) error {
	if s.replace != nil {
		return s.replace(ctx, oldJTI, newJTI, sessionID, ttl)
	}
	return nil
}

type authTokenGenStub struct{}

func (authTokenGenStub) RandomToken(int) (string, error) { return "token", nil }

type authTokenHasherStub struct{}

func (authTokenHasherStub) HashToken(token string) string { return "hash:" + token }

type authTokenRepoStub struct {
	findByHash             func(ctx context.Context, purpose, hash string) (*domain.AuthToken, error)
	create                 func(ctx context.Context, token domain.AuthToken) error
	consume                func(ctx context.Context, tokenID string, at time.Time) error
	deleteByUserAndPurpose func(ctx context.Context, userID, purpose string) error
}

func (s *authTokenRepoStub) FindByHash(ctx context.Context, purpose, hash string) (*domain.AuthToken, error) {
	if s.findByHash != nil {
		return s.findByHash(ctx, purpose, hash)
	}
	return nil, domain.ErrNotFound
}

func (s *authTokenRepoStub) Create(ctx context.Context, token domain.AuthToken) error {
	if s.create != nil {
		return s.create(ctx, token)
	}
	return nil
}

func (s *authTokenRepoStub) Consume(ctx context.Context, tokenID string, at time.Time) error {
	if s.consume != nil {
		return s.consume(ctx, tokenID, at)
	}
	return nil
}

func (s *authTokenRepoStub) DeleteByUserAndPurpose(ctx context.Context, userID, purpose string) error {
	if s.deleteByUserAndPurpose != nil {
		return s.deleteByUserAndPurpose(ctx, userID, purpose)
	}
	return nil
}
