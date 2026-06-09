package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Deps struct {
	UsersRead        ports.UserReadRepository
	UsersWrite       ports.UserWriteRepository
	ExternalsRead    ports.ExternalAccountReadRepository
	ExternalsWrite   ports.ExternalAccountWriteRepository
	SessionsRead     ports.SessionReadRepository
	SessionsWrite    ports.SessionWriteRepository
	Hasher           domain.PasswordHasher
	Tokens           ports.TokenManager
	OAuthState       ports.OAuthStateStore
	RefreshStore     ports.RefreshStore
	AuthTokens       ports.AuthTokenRepository
	TOTP             ports.TOTPRepository
	Providers        map[string]ports.OAuthProvider
	OAuthStateTTL    time.Duration
	MailSender         ports.Mailer
	RateLimiter        ports.RateLimiter
	IDGen              ports.IDGenerator
	TokenGen           ports.TokenGenerator
	TokenHasher        ports.TokenHasher
	PasswordValidator  ports.PasswordValidator
	TOTPVerifier       ports.TOTPCodeVerifier
	TOTPSecretGen      ports.TOTPSecretGenerator
	RecoveryCodeGen    ports.RecoveryCodeGenerator
	FrontendBaseURL  string
	VerificationTTL  time.Duration
	PasswordResetTTL time.Duration
	MFAChallengeTTL  time.Duration
	WorkspacesRead   ports.WorkspaceReadRepository
	WorkspacesWrite  ports.WorkspaceWriteRepository
	RolesRead        ports.RoleReadRepository
	RolesWrite       ports.RoleWriteRepository
	MembershipsRead  ports.MembershipReadRepository
	MembershipsWrite ports.MembershipWriteRepository
	InvitationsRead  ports.InvitationReadRepository
	InvitationsWrite ports.InvitationWriteRepository
	Logger           *slog.Logger
	UnitOfWork       ports.UnitOfWork
	OutboxWriter     ports.OutboxWriter
}

type SessionContext struct {
	Session domain.Session
	User    domain.User
	Tokens  ports.TokenPair
}

type LoginResult struct {
	SessionContext    *SessionContext
	User              *domain.User
	MFARequired       bool
	MFAChallengeToken string
}

type Provider struct {
	Provider    string `json:"provider"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`
}

type OAuthStartResult struct {
	Provider            string `json:"provider"`
	AuthorizationURL    string `json:"authorization_url"`
	State               string `json:"state"`
	CodeChallengeMethod string `json:"code_challenge_method,omitempty"`
}

type NewSessionInput struct {
	User   domain.User
	Method string
	IP     string
	UA     string
	Now    time.Time
}

type NewSession func(ctx context.Context, in NewSessionInput) (*SessionContext, error)
