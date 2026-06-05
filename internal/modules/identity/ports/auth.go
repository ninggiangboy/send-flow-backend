package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type UserReadRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, userID string) (*domain.User, error)
}

type UserWriteRepository interface {
	Create(ctx context.Context, user domain.User) error
	UpdatePassword(ctx context.Context, userID, hashedPassword string, at time.Time) error
	MarkEmailVerified(ctx context.Context, userID string, at time.Time) error
	SetMFAEnabledAt(ctx context.Context, userID string, enabledAt *time.Time, at time.Time) error
}

type ExternalAccountReadRepository interface {
	FindByProviderIdentity(ctx context.Context, provider, providerUserID string) (*domain.ExternalAuthAccount, error)
}

type ExternalAccountWriteRepository interface {
	Create(ctx context.Context, account domain.ExternalAuthAccount) error
	TouchLogin(ctx context.Context, accountID string, at time.Time) error
}

type SessionReadRepository interface {
	FindByID(ctx context.Context, sessionID string) (*domain.Session, error)
	FindByAccessJTI(ctx context.Context, jti string) (*domain.Session, error)
	ListByUser(ctx context.Context, userID string, now time.Time) ([]domain.Session, error)
}

type SessionWriteRepository interface {
	Create(ctx context.Context, session domain.Session) error
	RevokeByID(ctx context.Context, sessionID string, now time.Time) error
	RevokeByUser(ctx context.Context, userID string, now time.Time) error
	RotateTokens(ctx context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, now time.Time) error
}

type AccessClaims struct {
	Subject   string `json:"sub"`
	SessionID string `json:"sid"`
	JWTID     string `json:"jti"`
	Issuer    string `json:"iss"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	Type      string `json:"typ"`
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type TokenManager interface {
	Issue(userID, sessionID string, now time.Time) (TokenPair, string, string, error)
	ParseAccess(token string) (*AccessClaims, error)
	ParseRefresh(token string) (*AccessClaims, error)
}

type OAuthProvider interface {
	Name() string
	Type() string
	DisplayName() string
	Enabled() bool
	BuildAuthURL(state, redirectURI, codeChallenge string) string
	Exchange(ctx context.Context, code, redirectURI, codeVerifier string) (*domain.OAuthIdentity, error)
}

type OAuthState struct {
	Provider     string    `json:"provider"`
	RedirectURI  string    `json:"redirect_uri"`
	Intent       string    `json:"intent"`
	CodeVerifier string    `json:"code_verifier,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type OAuthStateStore interface {
	Save(ctx context.Context, state string, value OAuthState, ttl time.Duration) error
	GetAndDelete(ctx context.Context, state string) (*OAuthState, error)
}

type RefreshStore interface {
	Save(ctx context.Context, refreshJTI, sessionID string, ttl time.Duration) error
	Find(ctx context.Context, refreshJTI string) (string, error)
	Delete(ctx context.Context, refreshJTI string) error
	Replace(ctx context.Context, oldRefreshJTI, newRefreshJTI, sessionID string, ttl time.Duration) error
}

type AuthTokenRepository interface {
	Create(ctx context.Context, token domain.AuthToken) error
	FindByHash(ctx context.Context, purpose, tokenHash string) (*domain.AuthToken, error)
	Consume(ctx context.Context, tokenID string, at time.Time) error
	DeleteByUserAndPurpose(ctx context.Context, userID, purpose string) error
}

type TOTPRepository interface {
	UpsertSecret(ctx context.Context, secret domain.TOTPSecret) error
	FindSecretByUser(ctx context.Context, userID string) (*domain.TOTPSecret, error)
	DeleteSecret(ctx context.Context, userID string) error
	ReplaceRecoveryCodes(ctx context.Context, userID string, codes []domain.RecoveryCode) error
	ListRecoveryCodes(ctx context.Context, userID string) ([]domain.RecoveryCode, error)
	ConsumeRecoveryCode(ctx context.Context, codeID string, at time.Time) error
}
