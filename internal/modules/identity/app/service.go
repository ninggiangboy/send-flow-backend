package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/membership"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfa"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/oauth"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/session"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/verification"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/workspace"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	identityrediscache "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type AuditRecorder interface {
	Record(ctx context.Context, input RecordAuditInput) error
}

type RecordAuditInput struct {
	WorkspaceID    string
	ActorUserID    string
	ActionType     string
	TargetType     string
	TargetID       string
	PayloadSummary map[string]any
	RequestID      string
	OccurredAt     time.Time
}

type Options struct {
	UsersRead         ports.UserReadRepository
	UsersWrite        ports.UserWriteRepository
	ExternalsRead     ports.ExternalAccountReadRepository
	ExternalsWrite    ports.ExternalAccountWriteRepository
	SessionsRead      ports.SessionReadRepository
	SessionsWrite     ports.SessionWriteRepository
	Hasher            domain.PasswordHasher
	Tokens            ports.TokenManager
	OAuthState        ports.OAuthStateStore
	RefreshStore      ports.RefreshStore
	AuthTokens        ports.AuthTokenRepository
	TOTP              ports.TOTPRepository
	Providers         []ports.OAuthProvider
	OAuthStateTTL     time.Duration
	MailSender        ports.Mailer
	IDGen             ports.IDGenerator
	TokenGen          ports.TokenGenerator
	TokenHasher       ports.TokenHasher
	PasswordValidator ports.PasswordValidator
	TOTPVerifier      ports.TOTPCodeVerifier
	TOTPSecretGen     ports.TOTPSecretGenerator
	RecoveryCodeGen   ports.RecoveryCodeGenerator
	FrontendBaseURL   string
	VerificationTTL   time.Duration
	PasswordResetTTL  time.Duration
	MFAChallengeTTL   time.Duration
	WorkspacesRead    ports.WorkspaceReadRepository
	WorkspacesWrite   ports.WorkspaceWriteRepository
	RolesRead         ports.RoleReadRepository
	RolesWrite        ports.RoleWriteRepository
	MembershipsRead   ports.MembershipReadRepository
	MembershipsWrite  ports.MembershipWriteRepository
	InvitationsRead   ports.InvitationReadRepository
	InvitationsWrite  ports.InvitationWriteRepository
	SettingsRead      ports.WorkspaceSettingsReadRepository
	SettingsWrite     ports.WorkspaceSettingsWriteRepository
	AuditRecorder     AuditRecorder
	Logger            *slog.Logger
	UnitOfWork        ports.UnitOfWork
	OutboxWriter      ports.OutboxWriter
	RedisCache        *identityrediscache.Cache
}

type Service struct {
	commands        CommandBus
	queries         QueryBus
	settingsRead    ports.WorkspaceSettingsReadRepository
	settingsWrite   ports.WorkspaceSettingsWriteRepository
	membershipsRead ports.MembershipReadRepository
	rolesRead       ports.RoleReadRepository
	rolesWrite      ports.RoleWriteRepository
	idGen           ports.IDGenerator
	auditRecorder   AuditRecorder
	redisCache      *identityrediscache.Cache
	logger          *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.UnitOfWork == nil {
		panic("identity: UnitOfWork is required")
	}
	providerMap := map[string]ports.OAuthProvider{}
	for _, p := range opts.Providers {
		providerMap[p.Name()] = p
	}

	authTokenSvc := shared.NewAuthTokenService(opts.AuthTokens, opts.TokenGen, opts.IDGen, opts.TokenHasher)
	notifSvc := shared.NewNotificationService(opts.MailSender, opts.FrontendBaseURL)
	sessionFactory := shared.NewSessionFactory(opts.IDGen, opts.Tokens, opts.SessionsWrite, opts.RefreshStore, opts.Logger)

	signupH := auth.NewSignupHandler(auth.SignupOptions{
		UsersWrite:        opts.UsersWrite,
		Hasher:            opts.Hasher,
		PasswordValidator: opts.PasswordValidator,
		IdGen:             opts.IDGen,
		OutboxWriter:      opts.OutboxWriter,
		UnitOfWork:        opts.UnitOfWork,
		AuthTokens:        authTokenSvc,
		Notifications:     notifSvc,
		SessionFactory:    sessionFactory,
		VerificationTTL:   opts.VerificationTTL,
		Logger:            opts.Logger,
	})
	loginH := auth.NewLoginHandler(auth.LoginOptions{
		UsersWrite:      opts.UsersWrite,
		Hasher:          opts.Hasher,
		AuthTokens:      authTokenSvc,
		SessionFactory:  sessionFactory,
		MfaChallengeTTL: opts.MFAChallengeTTL,
		Logger:          opts.Logger,
	})
	listProviderH := auth.NewListProvidersHandler(providerMap)
	oauthStartH := oauth.NewStartHandler(oauth.StartOptions{
		Providers:     providerMap,
		IdGen:         opts.IDGen,
		OauthState:    opts.OAuthState,
		OauthStateTTL: opts.OAuthStateTTL,
		Logger:        opts.Logger,
	})
	oauthExchangeH := oauth.NewExchangeHandler(oauth.ExchangeOptions{
		Providers:      providerMap,
		OauthState:     opts.OAuthState,
		ExternalsWrite: opts.ExternalsWrite,
		UsersWrite:     opts.UsersWrite,
		IdGen:          opts.IDGen,
		UnitOfWork:     opts.UnitOfWork,
		SessionFactory: sessionFactory,
		Logger:         opts.Logger,
	})
	getMeH := auth.NewGetMeHandler(opts.UsersRead, opts.Logger)
	listSessionsH := session.NewListSessionsHandler(opts.SessionsRead, opts.Logger)
	revokeH := session.NewRevokeSessionHandler(session.Options{
		SessionsWrite: opts.SessionsWrite,
		RefreshStore:  opts.RefreshStore,
		Logger:        opts.Logger,
	})
	authnH := auth.NewAuthenticateHandler(opts.Tokens, opts.SessionsRead, opts.Logger)
	refreshH := auth.NewRefreshHandler(auth.RefreshOptions{
		Tokens:        opts.Tokens,
		RefreshStore:  opts.RefreshStore,
		SessionsWrite: opts.SessionsWrite,
		UsersWrite:    opts.UsersWrite,
		Logger:        opts.Logger,
	})
	reqVerifyH := verification.NewRequestEmailHandler(verification.RequestEmailOptions{
		UsersRead:       opts.UsersRead,
		AuthTokens:      authTokenSvc,
		Notifications:   notifSvc,
		VerificationTTL: opts.VerificationTTL,
		Logger:          opts.Logger,
	})
	verifyH := verification.NewVerifyEmailHandler(verification.VerifyEmailOptions{
		AuthTokens: authTokenSvc,
		UsersWrite: opts.UsersWrite,
		Logger:     opts.Logger,
	})
	forgotH := verification.NewForgotPasswordHandler(verification.ForgotPasswordOptions{
		UsersRead:        opts.UsersRead,
		AuthTokens:       authTokenSvc,
		Notifications:    notifSvc,
		PasswordResetTTL: opts.PasswordResetTTL,
		Logger:           opts.Logger,
	})
	resetH := verification.NewResetPasswordHandler(verification.ResetPasswordOptions{
		PasswordValidator: opts.PasswordValidator,
		AuthTokens:        authTokenSvc,
		Hasher:            opts.Hasher,
		UsersWrite:        opts.UsersWrite,
		SessionsWrite:     opts.SessionsWrite,
		AuthTokensRepo:    opts.AuthTokens,
		UnitOfWork:        opts.UnitOfWork,
		Logger:            opts.Logger,
	})
	mfaLoginH := mfa.NewMFALoginHandler(mfa.MFALoginOptions{
		AuthTokens:     authTokenSvc,
		UsersWrite:     opts.UsersWrite,
		Totp:           opts.TOTP,
		TokenHasher:    opts.TokenHasher,
		TotpVerifier:   opts.TOTPVerifier,
		SessionFactory: sessionFactory,
		Logger:         opts.Logger,
	})
	mfaSetupH := mfa.NewTOTPSetupHandler(mfa.TOTPSetupOptions{
		UsersWrite:    opts.UsersWrite,
		TotpSecretGen: opts.TOTPSecretGen,
		Totp:          opts.TOTP,
		Logger:        opts.Logger,
	})
	mfaEnableH := mfa.NewTOTPEnableHandler(mfa.TOTPEnableOptions{
		Totp:            opts.TOTP,
		TotpVerifier:    opts.TOTPVerifier,
		RecoveryCodeGen: opts.RecoveryCodeGen,
		IdGen:           opts.IDGen,
		TokenHasher:     opts.TokenHasher,
		UsersWrite:      opts.UsersWrite,
		UnitOfWork:      opts.UnitOfWork,
		Logger:          opts.Logger,
	})
	mfaDisableH := mfa.NewTOTPDisableHandler(mfa.TOTPDisableOptions{
		Totp:         opts.TOTP,
		TotpVerifier: opts.TOTPVerifier,
		UsersWrite:   opts.UsersWrite,
		UnitOfWork:   opts.UnitOfWork,
		Logger:       opts.Logger,
	})
	mfaRegenH := mfa.NewRegenerateHandler(mfaEnableH, opts.Logger)
	createWSH := workspace.NewCreateHandler(workspace.CreateOptions{
		IdGen:            opts.IDGen,
		WorkspacesWrite:  opts.WorkspacesWrite,
		MembershipsWrite: opts.MembershipsWrite,
		RolesWrite:       opts.RolesWrite,
		SettingsWrite:    opts.SettingsWrite,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	inviteMemberH := membership.NewInviteHandler(membership.InviteOptions{
		MembershipsWrite: opts.MembershipsWrite,
		RolesWrite:       opts.RolesWrite,
		UsersWrite:       opts.UsersWrite,
		IdGen:            opts.IDGen,
		InvitationsWrite: opts.InvitationsWrite,
		OutboxWriter:     opts.OutboxWriter,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	acceptInviteH := membership.NewAcceptInvitationHandler(membership.AcceptInvitationOptions{
		InvitationsWrite: opts.InvitationsWrite,
		UsersWrite:       opts.UsersWrite,
		MembershipsWrite: opts.MembershipsWrite,
		RolesWrite:       opts.RolesWrite,
		IdGen:            opts.IDGen,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	removeMemberH := membership.NewRemoveHandler(membership.RemoveOptions{
		MembershipsWrite: opts.MembershipsWrite,
		RolesWrite:       opts.RolesWrite,
		Logger:           opts.Logger,
	})
	updateRoleH := membership.NewUpdateRoleHandler(membership.UpdateRoleOptions{
		MembershipsWrite: opts.MembershipsWrite,
		RolesWrite:       opts.RolesWrite,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	listWSH := workspace.NewListHandler(opts.WorkspacesRead, opts.Logger)
	getWSH := workspace.NewGetHandler(opts.WorkspacesRead, opts.MembershipsRead, opts.Logger)
	listWSMembersH := membership.NewListMembersHandler(opts.MembershipsRead, opts.Logger)
	getWSAccessH := membership.NewGetAccessHandler(opts.MembershipsRead, opts.Logger)
	listWSInvitesH := membership.NewListInvitationsHandler(opts.MembershipsRead, opts.InvitationsRead, opts.Logger)

	return &Service{
		redisCache: opts.RedisCache,
		commands: newCommandBus(opts.Logger,
			signupH, loginH, oauthStartH, oauthExchangeH,
			revokeH, refreshH, reqVerifyH, verifyH, forgotH, resetH,
			mfaLoginH, mfaSetupH, mfaEnableH, mfaDisableH, mfaRegenH,
			createWSH, inviteMemberH, acceptInviteH, removeMemberH, updateRoleH,
		),
		queries: newQueryBus(opts.Logger,
			listProviderH, getMeH, listSessionsH, authnH,
			listWSH, getWSH, listWSMembersH, getWSAccessH, listWSInvitesH,
		),
		settingsRead:    opts.SettingsRead,
		settingsWrite:   opts.SettingsWrite,
		membershipsRead: opts.MembershipsRead,
		rolesRead:       opts.RolesRead,
		rolesWrite:      opts.RolesWrite,
		idGen:           opts.IDGen,
		auditRecorder:   opts.AuditRecorder,
		logger:          opts.Logger.With("usecase", "identity"),
	}
}

// ── Facade methods ──

func (s *Service) Signup(ctx context.Context, email, password, ip, ua string, now time.Time) (*SessionContext, error) {
	return s.commands.Signup(ctx, auth.SignupCommand{Email: email, Password: password, IP: ip, UA: ua, Now: now})
}

func (s *Service) Login(ctx context.Context, email, password, ip, ua string, now time.Time) (*shared.LoginResult, error) {
	return s.commands.Login(ctx, auth.LoginCommand{Email: email, Password: password, IP: ip, UA: ua, Now: now})
}

func (s *Service) ListProviders() []Provider { return s.queries.ListProviders() }

func (s *Service) OAuthStart(ctx context.Context, provider, redirectURI, intent, codeChallenge, codeVerifier string, now time.Time) (*OAuthStartResult, error) {
	return s.commands.OAuthStart(ctx, oauth.StartCommand{Provider: provider, RedirectURI: redirectURI, Intent: intent, CodeChallenge: codeChallenge, CodeVerifier: codeVerifier, Now: now})
}

func (s *Service) OAuthExchange(ctx context.Context, provider, code, state, redirectURI, codeVerifier, ip, ua string, now time.Time) (*SessionContext, *domain.OAuthIdentity, error) {
	return s.commands.OAuthExchange(ctx, oauth.ExchangeCommand{Provider: provider, Code: code, State: state, RedirectURI: redirectURI, CodeVerifier: codeVerifier, IP: ip, UA: ua, Now: now})
}

func (s *Service) GetMe(ctx context.Context, userID string) (*domain.User, error) {
	return s.queries.GetMe(ctx, userID)
}

func (s *Service) ListSessions(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	return s.queries.ListSessions(ctx, userID, now)
}

func (s *Service) RevokeSession(ctx context.Context, sessionID, userID string, now time.Time) error {
	return s.commands.RevokeSession(ctx, sessionID, userID, now)
}

func (s *Service) AuthenticateAccessToken(ctx context.Context, token string, now time.Time) (*domain.Session, *domain.User, error) {
	return s.queries.AuthenticateAccessToken(ctx, token, now)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string, now time.Time) (*SessionContext, error) {
	return s.commands.Refresh(ctx, auth.RefreshCommand{RefreshToken: refreshToken, Now: now})
}

func (s *Service) RequestEmailVerification(ctx context.Context, userID string, now time.Time) error {
	return s.commands.RequestEmailVerification(ctx, userID, now)
}

func (s *Service) VerifyEmail(ctx context.Context, token string, now time.Time) error {
	return s.commands.VerifyEmail(ctx, token, now)
}

func (s *Service) ForgotPassword(ctx context.Context, email string, now time.Time) error {
	return s.commands.ForgotPassword(ctx, email, now)
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string, now time.Time) error {
	return s.commands.ResetPassword(ctx, verification.ResetPasswordCommand{Token: token, NewPassword: newPassword, Now: now})
}

func (s *Service) MFALogin(ctx context.Context, challengeToken, code, recoveryCode, ip, ua string, now time.Time) (*SessionContext, error) {
	return s.commands.MFALogin(ctx, mfa.MFALoginCommand{ChallengeToken: challengeToken, Code: code, RecoveryCode: recoveryCode, IP: ip, UA: ua, Now: now})
}

func (s *Service) MFATOTPSetup(ctx context.Context, userID string, now time.Time) (*mfa.TOTPSetupResult, error) {
	return s.commands.MFATOTPSetup(ctx, userID, now)
}

func (s *Service) MFATOTPEnable(ctx context.Context, userID, code string, now time.Time) (*mfa.TOTPEnableResult, error) {
	return s.commands.MFATOTPEnable(ctx, userID, code, now)
}

func (s *Service) MFATOTPDisable(ctx context.Context, userID, code string, now time.Time) error {
	return s.commands.MFATOTPDisable(ctx, mfa.TOTPDisableCommand{UserID: userID, Code: code, Now: now})
}

func (s *Service) MFARegenerate(ctx context.Context, userID, code string, now time.Time) (*mfa.TOTPEnableResult, error) {
	return s.commands.MFARegenerate(ctx, userID, code, now)
}

// Workspace commands
func (s *Service) CreateWorkspace(ctx context.Context, name, userID string, now time.Time) (*domain.Workspace, error) {
	return s.commands.CreateWorkspace(ctx, workspace.CreateCommand{Name: name, UserID: userID, Now: now})
}

func (s *Service) AcceptWorkspaceInvitation(ctx context.Context, token, userID string, now time.Time) (*domain.Membership, error) {
	return s.commands.AcceptWorkspaceInvitation(ctx, membership.AcceptInvitationCommand{Token: token, UserID: userID, Now: now})
}

func (s *Service) RemoveWorkspaceMember(ctx context.Context, workspaceID, membershipID, removerID string, now time.Time) error {
	return s.commands.RemoveWorkspaceMember(ctx, membership.RemoveCommand{WorkspaceID: workspaceID, MembershipID: membershipID, RemoverID: removerID, Now: now})
}
