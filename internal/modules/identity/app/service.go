package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/acceptworkspaceinvitation"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/authenticate"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/createworkspace"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/forgotpassword"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/getme"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/getworkspace"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/getworkspaceaccess"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/inviteworkspacemember"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/listproviders"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/listsessions"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/listworkspaceinvitations"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/listworkspacemembers"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/listworkspaces"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/login"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfalogin"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfaregenerate"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfatotpdisable"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfatotpenable"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfatotpsetup"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/oauthexchange"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/oauthstart"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/refresh"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/removeworkspacemember"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/requestemailverification"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/resetpassword"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/revokesession"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/signup"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/updateworkspacememberrole"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/verifyemail"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
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
	RateLimiter       ports.RateLimiter
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
	logger          *slog.Logger
}

type SessionContext = usecase.SessionContext
type Provider = usecase.Provider
type OAuthStartResult = usecase.OAuthStartResult

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

	authTokenSvc := usecase.NewAuthTokenService(opts.AuthTokens, opts.TokenGen, opts.IDGen, opts.TokenHasher)
	notifSvc := usecase.NewNotificationService(opts.MailSender, opts.FrontendBaseURL)
	sessionFactory := usecase.NewSessionFactory(opts.IDGen, opts.Tokens, opts.SessionsWrite, opts.RefreshStore, opts.Logger)

	signupH := signup.New(signup.Options{
		UsersRead:         opts.UsersRead,
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
	loginH := login.New(login.Options{
		UsersRead:       opts.UsersRead,
		Hasher:          opts.Hasher,
		AuthTokens:      authTokenSvc,
		SessionFactory:  sessionFactory,
		MfaChallengeTTL: opts.MFAChallengeTTL,
		Logger:          opts.Logger,
	})
	listProviderH := listproviders.New(providerMap)
	oauthStartH := oauthstart.New(oauthstart.Options{
		Providers:     providerMap,
		IdGen:         opts.IDGen,
		OauthState:    opts.OAuthState,
		OauthStateTTL: opts.OAuthStateTTL,
		Logger:        opts.Logger,
	})
	oauthExchangeH := oauthexchange.New(oauthexchange.Options{
		Providers:      providerMap,
		OauthState:     opts.OAuthState,
		ExternalsRead:  opts.ExternalsRead,
		ExternalsWrite: opts.ExternalsWrite,
		UsersRead:      opts.UsersRead,
		UsersWrite:     opts.UsersWrite,
		IdGen:          opts.IDGen,
		UnitOfWork:     opts.UnitOfWork,
		SessionFactory: sessionFactory,
		Logger:         opts.Logger,
	})
	getMeH := getme.New(opts.UsersRead, opts.Logger)
	listSessionsH := listsessions.New(opts.SessionsRead, opts.Logger)
	revokeH := revokesession.New(revokesession.Options{
		SessionsRead:  opts.SessionsRead,
		SessionsWrite: opts.SessionsWrite,
		RefreshStore:  opts.RefreshStore,
		Logger:        opts.Logger,
	})
	authnH := authenticate.New(opts.Tokens, opts.Logger)
	refreshH := refresh.New(refresh.Options{
		Tokens:        opts.Tokens,
		RefreshStore:  opts.RefreshStore,
		SessionsRead:  opts.SessionsRead,
		SessionsWrite: opts.SessionsWrite,
		UsersRead:     opts.UsersRead,
		Logger:        opts.Logger,
	})
	reqVerifyH := requestemailverification.New(requestemailverification.Options{
		UsersRead:       opts.UsersRead,
		AuthTokens:      authTokenSvc,
		Notifications:   notifSvc,
		VerificationTTL: opts.VerificationTTL,
		Logger:          opts.Logger,
	})
	verifyH := verifyemail.New(verifyemail.Options{
		AuthTokens: authTokenSvc,
		UsersWrite: opts.UsersWrite,
		Logger:     opts.Logger,
	})
	forgotH := forgotpassword.New(forgotpassword.Options{
		UsersRead:        opts.UsersRead,
		AuthTokens:       authTokenSvc,
		Notifications:    notifSvc,
		PasswordResetTTL: opts.PasswordResetTTL,
		Logger:           opts.Logger,
	})
	resetH := resetpassword.New(resetpassword.Options{
		PasswordValidator: opts.PasswordValidator,
		AuthTokens:        authTokenSvc,
		Hasher:            opts.Hasher,
		UsersWrite:        opts.UsersWrite,
		SessionsWrite:     opts.SessionsWrite,
		AuthTokensRepo:    opts.AuthTokens,
		Logger:            opts.Logger,
	})
	mfaLoginH := mfalogin.New(mfalogin.Options{
		AuthTokens:     authTokenSvc,
		UsersRead:      opts.UsersRead,
		Totp:           opts.TOTP,
		TokenHasher:    opts.TokenHasher,
		TotpVerifier:   opts.TOTPVerifier,
		SessionFactory: sessionFactory,
		Logger:         opts.Logger,
	})
	mfaSetupH := mfatotpsetup.New(mfatotpsetup.Options{
		UsersRead:     opts.UsersRead,
		TotpSecretGen: opts.TOTPSecretGen,
		Totp:          opts.TOTP,
		Logger:        opts.Logger,
	})
	mfaEnableH := mfatotpenable.New(mfatotpenable.Options{
		Totp:            opts.TOTP,
		TotpVerifier:    opts.TOTPVerifier,
		RecoveryCodeGen: opts.RecoveryCodeGen,
		IdGen:           opts.IDGen,
		TokenHasher:     opts.TokenHasher,
		UsersWrite:      opts.UsersWrite,
		UnitOfWork:      opts.UnitOfWork,
		Logger:          opts.Logger,
	})
	mfaDisableH := mfatotpdisable.New(mfatotpdisable.Options{
		UsersRead:    opts.UsersRead,
		Hasher:       opts.Hasher,
		Totp:         opts.TOTP,
		TotpVerifier: opts.TOTPVerifier,
		UsersWrite:   opts.UsersWrite,
		UnitOfWork:   opts.UnitOfWork,
		Logger:       opts.Logger,
	})
	mfaRegenH := mfaregenerate.New(mfaEnableH, opts.Logger)
	createWSH := createworkspace.New(createworkspace.Options{
		IdGen:            opts.IDGen,
		WorkspacesWrite:  opts.WorkspacesWrite,
		MembershipsWrite: opts.MembershipsWrite,
		RolesWrite:       opts.RolesWrite,
		SettingsWrite:    opts.SettingsWrite,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	inviteMemberH := inviteworkspacemember.New(inviteworkspacemember.Options{
		MembershipsRead:  opts.MembershipsRead,
		MembershipsWrite: opts.MembershipsWrite,
		RolesRead:        opts.RolesRead,
		RolesWrite:       opts.RolesWrite,
		UsersRead:        opts.UsersRead,
		IdGen:            opts.IDGen,
		InvitationsWrite: opts.InvitationsWrite,
		OutboxWriter:     opts.OutboxWriter,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	acceptInviteH := acceptworkspaceinvitation.New(acceptworkspaceinvitation.Options{
		InvitationsRead:  opts.InvitationsRead,
		InvitationsWrite: opts.InvitationsWrite,
		UsersRead:        opts.UsersRead,
		MembershipsRead:  opts.MembershipsRead,
		MembershipsWrite: opts.MembershipsWrite,
		RolesRead:        opts.RolesRead,
		RolesWrite:       opts.RolesWrite,
		IdGen:            opts.IDGen,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	removeMemberH := removeworkspacemember.New(removeworkspacemember.Options{
		MembershipsRead:  opts.MembershipsRead,
		MembershipsWrite: opts.MembershipsWrite,
		RolesRead:        opts.RolesRead,
		Logger:           opts.Logger,
	})
	updateRoleH := updateworkspacememberrole.New(updateworkspacememberrole.Options{
		MembershipsRead:  opts.MembershipsRead,
		MembershipsWrite: opts.MembershipsWrite,
		RolesRead:        opts.RolesRead,
		RolesWrite:       opts.RolesWrite,
		UnitOfWork:       opts.UnitOfWork,
		Logger:           opts.Logger,
	})
	listWSH := listworkspaces.New(opts.WorkspacesRead, opts.Logger)
	getWSH := getworkspace.New(opts.WorkspacesRead, opts.MembershipsRead, opts.Logger)
	listWSMembersH := listworkspacemembers.New(opts.MembershipsRead, opts.Logger)
	getWSAccessH := getworkspaceaccess.New(opts.MembershipsRead, opts.Logger)
	listWSInvitesH := listworkspaceinvitations.New(opts.MembershipsRead, opts.InvitationsRead, opts.Logger)

	return &Service{
		commands:        newCommandBus(opts.Logger, signupH, loginH, oauthStartH, oauthExchangeH, revokeH, refreshH, reqVerifyH, verifyH, forgotH, resetH, mfaLoginH, mfaSetupH, mfaEnableH, mfaDisableH, mfaRegenH, createWSH, inviteMemberH, acceptInviteH, removeMemberH, updateRoleH),
		queries:         newQueryBus(opts.Logger, listProviderH, getMeH, listSessionsH, authnH, listWSH, getWSH, listWSMembersH, getWSAccessH, listWSInvitesH),
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

func (s *Service) Signup(ctx context.Context, email, password, ip, ua string, now time.Time) (*SessionContext, error) {
	return s.commands.Signup(ctx, signup.Command{Email: email, Password: password, IP: ip, UA: ua, Now: now})
}
func (s *Service) Login(ctx context.Context, email, password, ip, ua string, now time.Time) (*usecase.LoginResult, error) {
	return s.commands.Login(ctx, login.Command{Email: email, Password: password, IP: ip, UA: ua, Now: now})
}
func (s *Service) ListProviders() []Provider { return s.queries.ListProviders() }
func (s *Service) OAuthStart(ctx context.Context, provider, redirectURI, intent, codeChallenge, codeVerifier string, now time.Time) (*OAuthStartResult, error) {
	return s.commands.OAuthStart(ctx, oauthstart.Command{Provider: provider, RedirectURI: redirectURI, Intent: intent, CodeChallenge: codeChallenge, CodeVerifier: codeVerifier, Now: now})
}
func (s *Service) OAuthExchange(ctx context.Context, provider, code, state, redirectURI, codeVerifier, ip, ua string, now time.Time) (*SessionContext, *domain.OAuthIdentity, error) {
	return s.commands.OAuthExchange(ctx, oauthexchange.Command{Provider: provider, Code: code, State: state, RedirectURI: redirectURI, CodeVerifier: codeVerifier, IP: ip, UA: ua, Now: now})
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
func (s *Service) AuthenticateAccessToken(ctx context.Context, token string) (*domain.Session, *domain.User, error) {
	return s.queries.AuthenticateAccessToken(ctx, token)
}
func (s *Service) Refresh(ctx context.Context, refreshToken string, now time.Time) (*SessionContext, error) {
	return s.commands.Refresh(ctx, refresh.Command{RefreshToken: refreshToken, Now: now})
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
	return s.commands.ResetPassword(ctx, resetpassword.Command{Token: token, NewPassword: newPassword, Now: now})
}
func (s *Service) MFALogin(ctx context.Context, challengeToken, code, recoveryCode, ip, ua string, now time.Time) (*SessionContext, error) {
	return s.commands.MFALogin(ctx, mfalogin.Command{ChallengeToken: challengeToken, Code: code, RecoveryCode: recoveryCode, IP: ip, UA: ua, Now: now})
}
func (s *Service) MFATOTPSetup(ctx context.Context, userID string, now time.Time) (*mfatotpsetup.Result, error) {
	return s.commands.MFATOTPSetup(ctx, userID, now)
}
func (s *Service) MFATOTPEnable(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error) {
	return s.commands.MFATOTPEnable(ctx, userID, code, now)
}
func (s *Service) MFATOTPDisable(ctx context.Context, userID, password, code string, now time.Time) error {
	return s.commands.MFATOTPDisable(ctx, mfatotpdisable.Command{UserID: userID, Password: password, Code: code, Now: now})
}
func (s *Service) MFARegenerate(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error) {
	return s.commands.MFARegenerate(ctx, userID, code, now)
}

// Workspace commands
func (s *Service) CreateWorkspace(ctx context.Context, name, userID string, now time.Time) (*domain.Workspace, error) {
	return s.commands.CreateWorkspace(ctx, createworkspace.Command{Name: name, UserID: userID, Now: now})
}
func (s *Service) AcceptWorkspaceInvitation(ctx context.Context, token, userID string, now time.Time) (*domain.Membership, error) {
	return s.commands.AcceptWorkspaceInvitation(ctx, acceptworkspaceinvitation.Command{Token: token, UserID: userID, Now: now})
}
func (s *Service) RemoveWorkspaceMember(ctx context.Context, workspaceID, membershipID, removerID string, now time.Time) error {
	return s.commands.RemoveWorkspaceMember(ctx, removeworkspacemember.Command{WorkspaceID: workspaceID, MembershipID: membershipID, RemoverID: removerID, Now: now})
}
