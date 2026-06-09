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
	"github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/ratelimit"
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
	Providers        []ports.OAuthProvider
	OAuthStateTTL    time.Duration
	MailSender       email.Sender
	RateLimiter      ratelimit.Service
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
	SettingsRead     ports.WorkspaceSettingsReadRepository
	SettingsWrite    ports.WorkspaceSettingsWriteRepository
	AuditRecorder    AuditRecorder
	Logger           *slog.Logger
	UnitOfWork       ports.UnitOfWork
}

type Service struct {
	commands      CommandBus
	queries       QueryBus
	deps          usecase.Deps
	settingsRead  ports.WorkspaceSettingsReadRepository
	settingsWrite ports.WorkspaceSettingsWriteRepository
	auditRecorder AuditRecorder
	logger        *slog.Logger
}

type SessionContext = usecase.SessionContext
type Provider = usecase.Provider
type OAuthStartResult = usecase.OAuthStartResult

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	providerMap := map[string]ports.OAuthProvider{}
	for _, p := range opts.Providers {
		providerMap[p.Name()] = p
	}
	deps := usecase.Deps{
		UsersRead:        opts.UsersRead,
		UsersWrite:       opts.UsersWrite,
		ExternalsRead:    opts.ExternalsRead,
		ExternalsWrite:   opts.ExternalsWrite,
		SessionsRead:     opts.SessionsRead,
		SessionsWrite:    opts.SessionsWrite,
		Hasher:           opts.Hasher,
		Tokens:           opts.Tokens,
		OAuthState:       opts.OAuthState,
		RefreshStore:     opts.RefreshStore,
		AuthTokens:       opts.AuthTokens,
		TOTP:             opts.TOTP,
		Providers:        providerMap,
		OAuthStateTTL:    opts.OAuthStateTTL,
		MailSender:       opts.MailSender,
		RateLimiter:      opts.RateLimiter,
		FrontendBaseURL:  opts.FrontendBaseURL,
		VerificationTTL:  opts.VerificationTTL,
		PasswordResetTTL: opts.PasswordResetTTL,
		MFAChallengeTTL:  opts.MFAChallengeTTL,
		WorkspacesRead:   opts.WorkspacesRead,
		WorkspacesWrite:  opts.WorkspacesWrite,
		RolesRead:        opts.RolesRead,
		RolesWrite:       opts.RolesWrite,
		MembershipsRead:  opts.MembershipsRead,
		MembershipsWrite: opts.MembershipsWrite,
		InvitationsRead:  opts.InvitationsRead,
		InvitationsWrite: opts.InvitationsWrite,
		Logger:           opts.Logger,
		UnitOfWork:       opts.UnitOfWork,
	}
	newSession := usecase.BuildNewSession(deps)
	signupH := signup.New(deps, newSession)
	loginH := login.New(deps, newSession)
	listProviderH := listproviders.New(deps)
	oauthStartH := oauthstart.New(deps)
	oauthExchangeH := oauthexchange.New(deps, newSession)
	getMeH := getme.New(deps)
	listSessionsH := listsessions.New(deps)
	revokeH := revokesession.New(deps)
	authnH := authenticate.New(deps)
	refreshH := refresh.New(deps)
	reqVerifyH := requestemailverification.New(deps)
	verifyH := verifyemail.New(deps)
	forgotH := forgotpassword.New(deps)
	resetH := resetpassword.New(deps)
	mfaLoginH := mfalogin.New(deps, newSession)
	mfaSetupH := mfatotpsetup.New(deps)
	mfaEnableH := mfatotpenable.New(deps)
	mfaDisableH := mfatotpdisable.New(deps)
	mfaRegenH := mfaregenerate.New(deps)
	createWSH := createworkspace.New(deps)
	inviteMemberH := inviteworkspacemember.New(deps)
	acceptInviteH := acceptworkspaceinvitation.New(deps)
	removeMemberH := removeworkspacemember.New(deps)
	updateRoleH := updateworkspacememberrole.New(deps)
	listWSH := listworkspaces.New(deps)
	getWSH := getworkspace.New(deps)
	listWSMembersH := listworkspacemembers.New(deps)
	getWSAccessH := getworkspaceaccess.New(deps)
	listWSInvitesH := listworkspaceinvitations.New(deps)

	return &Service{
		commands:      newCommandBus(signupH, loginH, oauthStartH, oauthExchangeH, revokeH, refreshH, reqVerifyH, verifyH, forgotH, resetH, mfaLoginH, mfaSetupH, mfaEnableH, mfaDisableH, mfaRegenH, createWSH, inviteMemberH, acceptInviteH, removeMemberH, updateRoleH),
		queries:       newQueryBus(listProviderH, getMeH, listSessionsH, authnH, listWSH, getWSH, listWSMembersH, getWSAccessH, listWSInvitesH),
		deps:          deps,
		settingsRead:  opts.SettingsRead,
		settingsWrite: opts.SettingsWrite,
		auditRecorder: opts.AuditRecorder,
		logger:        opts.Logger.With("usecase", "identity"),
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
