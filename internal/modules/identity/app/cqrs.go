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
)

type CommandBus interface {
	Signup(ctx context.Context, cmd auth.SignupCommand) (*shared.SessionContext, error)
	Login(ctx context.Context, cmd auth.LoginCommand) (*shared.LoginResult, error)
	OAuthStart(ctx context.Context, cmd oauth.StartCommand) (*shared.OAuthStartResult, error)
	OAuthExchange(ctx context.Context, cmd oauth.ExchangeCommand) (*shared.SessionContext, *domain.OAuthIdentity, error)
	RevokeSession(ctx context.Context, sessionID, userID string, now time.Time) error
	Refresh(ctx context.Context, cmd auth.RefreshCommand) (*shared.SessionContext, error)
	RequestEmailVerification(ctx context.Context, userID string, now time.Time) error
	VerifyEmail(ctx context.Context, token string, now time.Time) error
	ForgotPassword(ctx context.Context, email string, now time.Time) error
	ResetPassword(ctx context.Context, cmd verification.ResetPasswordCommand) error
	MFALogin(ctx context.Context, cmd mfa.MFALoginCommand) (*shared.SessionContext, error)
	MFATOTPSetup(ctx context.Context, userID string, now time.Time) (*mfa.TOTPSetupResult, error)
	MFATOTPEnable(ctx context.Context, userID, code string, now time.Time) (*mfa.TOTPEnableResult, error)
	MFATOTPDisable(ctx context.Context, cmd mfa.TOTPDisableCommand) error
	MFARegenerate(ctx context.Context, userID, code string, now time.Time) (*mfa.TOTPEnableResult, error)
	CreateWorkspace(ctx context.Context, cmd workspace.CreateCommand) (*domain.Workspace, error)
	InviteWorkspaceMember(ctx context.Context, cmd membership.InviteCommand) (*membership.InviteResult, error)
	AcceptWorkspaceInvitation(ctx context.Context, cmd membership.AcceptInvitationCommand) (*domain.Membership, error)
	RemoveWorkspaceMember(ctx context.Context, cmd membership.RemoveCommand) error
	UpdateWorkspaceMemberRole(ctx context.Context, cmd membership.UpdateRoleCommand) error
}

type QueryBus interface {
	ListProviders() []shared.Provider
	GetMe(ctx context.Context, userID string) (*domain.User, error)
	ListSessions(ctx context.Context, userID string, now time.Time) ([]domain.Session, error)
	AuthenticateAccessToken(ctx context.Context, token string, now time.Time) (*domain.Session, *domain.User, error)
	ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error)
	ListWorkspaceMembers(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error)
	GetWorkspaceAccess(ctx context.Context, workspaceID, userID string) (*domain.Membership, error)
	ListWorkspaceInvitations(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error)
}

type commandBus struct {
	logger       *slog.Logger
	signup       *auth.SignupHandler
	login        *auth.LoginHandler
	oauthStart   *oauth.StartHandler
	oauthEx      *oauth.ExchangeHandler
	revoke       *session.RevokeSessionHandler
	refresh      *auth.RefreshHandler
	reqVerify    *verification.RequestEmailHandler
	verify       *verification.VerifyEmailHandler
	forgot       *verification.ForgotPasswordHandler
	reset        *verification.ResetPasswordHandler
	mfaLogin     *mfa.MFALoginHandler
	mfaSetup     *mfa.TOTPSetupHandler
	mfaEnable    *mfa.TOTPEnableHandler
	mfaDisable   *mfa.TOTPDisableHandler
	mfaRegen     *mfa.RegenerateHandler
	createWS     *workspace.CreateHandler
	inviteMember *membership.InviteHandler
	acceptInvite *membership.AcceptInvitationHandler
	removeMember *membership.RemoveHandler
	updateRole   *membership.UpdateRoleHandler
}

func newCommandBus(
	logger *slog.Logger,
	signupH *auth.SignupHandler,
	loginH *auth.LoginHandler,
	oauthStartH *oauth.StartHandler,
	oauthExchangeH *oauth.ExchangeHandler,
	revokeH *session.RevokeSessionHandler,
	refreshH *auth.RefreshHandler,
	reqVerifyH *verification.RequestEmailHandler,
	verifyH *verification.VerifyEmailHandler,
	forgotH *verification.ForgotPasswordHandler,
	resetH *verification.ResetPasswordHandler,
	mfaLoginH *mfa.MFALoginHandler,
	mfaSetupH *mfa.TOTPSetupHandler,
	mfaEnableH *mfa.TOTPEnableHandler,
	mfaDisableH *mfa.TOTPDisableHandler,
	mfaRegenH *mfa.RegenerateHandler,
	createWSH *workspace.CreateHandler,
	inviteMemberH *membership.InviteHandler,
	acceptInviteH *membership.AcceptInvitationHandler,
	removeMemberH *membership.RemoveHandler,
	updateRoleH *membership.UpdateRoleHandler,
) CommandBus {
	return &commandBus{
		logger:       logger,
		signup:       signupH,
		login:        loginH,
		oauthStart:   oauthStartH,
		oauthEx:      oauthExchangeH,
		revoke:       revokeH,
		refresh:      refreshH,
		reqVerify:    reqVerifyH,
		verify:       verifyH,
		forgot:       forgotH,
		reset:        resetH,
		mfaLogin:     mfaLoginH,
		mfaSetup:     mfaSetupH,
		mfaEnable:    mfaEnableH,
		mfaDisable:   mfaDisableH,
		mfaRegen:     mfaRegenH,
		createWS:     createWSH,
		inviteMember: inviteMemberH,
		acceptInvite: acceptInviteH,
		removeMember: removeMemberH,
		updateRole:   updateRoleH,
	}
}

func (b *commandBus) Signup(ctx context.Context, cmd auth.SignupCommand) (*shared.SessionContext, error) {
	b.logger.Info("dispatching command", "command", "signup")
	result, err := b.signup.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "signup", "error", err)
	}
	return result, err
}

func (b *commandBus) Login(ctx context.Context, cmd auth.LoginCommand) (*shared.LoginResult, error) {
	b.logger.Info("dispatching command", "command", "login")
	result, err := b.login.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "login", "error", err)
	}
	return result, err
}

func (b *commandBus) OAuthStart(ctx context.Context, cmd oauth.StartCommand) (*shared.OAuthStartResult, error) {
	b.logger.Info("dispatching command", "command", "oauth_start")
	result, err := b.oauthStart.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "oauth_start", "error", err)
	}
	return result, err
}

func (b *commandBus) OAuthExchange(ctx context.Context, cmd oauth.ExchangeCommand) (*shared.SessionContext, *domain.OAuthIdentity, error) {
	b.logger.Info("dispatching command", "command", "oauth_exchange")
	session, identity, err := b.oauthEx.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "oauth_exchange", "error", err)
	}
	return session, identity, err
}

func (b *commandBus) RevokeSession(ctx context.Context, sessionID, userID string, now time.Time) error {
	b.logger.Info("dispatching command", "command", "revoke_session")
	err := b.revoke.Execute(ctx, sessionID, userID, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "revoke_session", "error", err)
	}
	return err
}

func (b *commandBus) Refresh(ctx context.Context, cmd auth.RefreshCommand) (*shared.SessionContext, error) {
	b.logger.Info("dispatching command", "command", "refresh")
	result, err := b.refresh.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "refresh", "error", err)
	}
	return result, err
}

func (b *commandBus) RequestEmailVerification(ctx context.Context, userID string, now time.Time) error {
	b.logger.Info("dispatching command", "command", "request_email_verification")
	err := b.reqVerify.Execute(ctx, userID, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "request_email_verification", "error", err)
	}
	return err
}

func (b *commandBus) VerifyEmail(ctx context.Context, token string, now time.Time) error {
	b.logger.Info("dispatching command", "command", "verify_email")
	err := b.verify.Execute(ctx, token, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "verify_email", "error", err)
	}
	return err
}

func (b *commandBus) ForgotPassword(ctx context.Context, email string, now time.Time) error {
	b.logger.Info("dispatching command", "command", "forgot_password")
	err := b.forgot.Execute(ctx, email, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "forgot_password", "error", err)
	}
	return err
}

func (b *commandBus) ResetPassword(ctx context.Context, cmd verification.ResetPasswordCommand) error {
	b.logger.Info("dispatching command", "command", "reset_password")
	err := b.reset.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "reset_password", "error", err)
	}
	return err
}

func (b *commandBus) MFALogin(ctx context.Context, cmd mfa.MFALoginCommand) (*shared.SessionContext, error) {
	b.logger.Info("dispatching command", "command", "mfa_login")
	result, err := b.mfaLogin.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_login", "error", err)
	}
	return result, err
}

func (b *commandBus) MFATOTPSetup(ctx context.Context, userID string, now time.Time) (*mfa.TOTPSetupResult, error) {
	b.logger.Info("dispatching command", "command", "mfa_totp_setup")
	result, err := b.mfaSetup.Execute(ctx, userID, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_totp_setup", "error", err)
	}
	return result, err
}

func (b *commandBus) MFATOTPEnable(ctx context.Context, userID, code string, now time.Time) (*mfa.TOTPEnableResult, error) {
	b.logger.Info("dispatching command", "command", "mfa_totp_enable")
	result, err := b.mfaEnable.Execute(ctx, userID, code, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_totp_enable", "error", err)
	}
	return result, err
}

func (b *commandBus) MFATOTPDisable(ctx context.Context, cmd mfa.TOTPDisableCommand) error {
	b.logger.Info("dispatching command", "command", "mfa_totp_disable")
	err := b.mfaDisable.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_totp_disable", "error", err)
	}
	return err
}

func (b *commandBus) MFARegenerate(ctx context.Context, userID, code string, now time.Time) (*mfa.TOTPEnableResult, error) {
	b.logger.Info("dispatching command", "command", "mfa_regenerate")
	result, err := b.mfaRegen.Execute(ctx, userID, code, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_regenerate", "error", err)
	}
	return result, err
}

func (b *commandBus) CreateWorkspace(ctx context.Context, cmd workspace.CreateCommand) (*domain.Workspace, error) {
	b.logger.Info("dispatching command", "command", "create_workspace")
	result, err := b.createWS.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "create_workspace", "error", err)
	}
	return result, err
}

func (b *commandBus) InviteWorkspaceMember(ctx context.Context, cmd membership.InviteCommand) (*membership.InviteResult, error) {
	b.logger.Info("dispatching command", "command", "invite_workspace_member")
	result, err := b.inviteMember.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "invite_workspace_member", "error", err)
	}
	return result, err
}

func (b *commandBus) AcceptWorkspaceInvitation(ctx context.Context, cmd membership.AcceptInvitationCommand) (*domain.Membership, error) {
	b.logger.Info("dispatching command", "command", "accept_workspace_invitation")
	result, err := b.acceptInvite.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "accept_workspace_invitation", "error", err)
	}
	return result, err
}

func (b *commandBus) RemoveWorkspaceMember(ctx context.Context, cmd membership.RemoveCommand) error {
	b.logger.Info("dispatching command", "command", "remove_workspace_member")
	err := b.removeMember.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "remove_workspace_member", "error", err)
	}
	return err
}

func (b *commandBus) UpdateWorkspaceMemberRole(ctx context.Context, cmd membership.UpdateRoleCommand) error {
	b.logger.Info("dispatching command", "command", "update_workspace_member_role")
	err := b.updateRole.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "update_workspace_member_role", "error", err)
	}
	return err
}

type queryBus struct {
	logger         *slog.Logger
	listProviders  *auth.ListProvidersHandler
	getMe          *auth.GetMeHandler
	listSessions   *session.ListSessionsHandler
	authenticate   *auth.AuthenticateHandler
	listWorkspaces *workspace.ListHandler
	getWorkspace   *workspace.GetHandler
	listMembers    *membership.ListMembersHandler
	getAccess      *membership.GetAccessHandler
	listInvites    *membership.ListInvitationsHandler
}

func newQueryBus(
	logger *slog.Logger,
	listProviderH *auth.ListProvidersHandler,
	getMeH *auth.GetMeHandler,
	listSessionsH *session.ListSessionsHandler,
	authnH *auth.AuthenticateHandler,
	listWSH *workspace.ListHandler,
	getWSH *workspace.GetHandler,
	listWSMembersH *membership.ListMembersHandler,
	getWSAccessH *membership.GetAccessHandler,
	listWSInvitesH *membership.ListInvitationsHandler,
) QueryBus {
	return &queryBus{
		logger:         logger,
		listProviders:  listProviderH,
		getMe:          getMeH,
		listSessions:   listSessionsH,
		authenticate:   authnH,
		listWorkspaces: listWSH,
		getWorkspace:   getWSH,
		listMembers:    listWSMembersH,
		getAccess:      getWSAccessH,
		listInvites:    listWSInvitesH,
	}
}

func (b *queryBus) ListProviders() []shared.Provider {
	b.logger.Info("dispatching query", "query", "list_providers")
	return b.listProviders.Execute()
}

func (b *queryBus) GetMe(ctx context.Context, userID string) (*domain.User, error) {
	b.logger.Info("dispatching query", "query", "get_me")
	user, err := b.getMe.Execute(ctx, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_me", "error", err)
	}
	return user, err
}

func (b *queryBus) ListSessions(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	b.logger.Info("dispatching query", "query", "list_sessions")
	sessions, err := b.listSessions.Execute(ctx, userID, now)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_sessions", "error", err)
	}
	return sessions, err
}

func (b *queryBus) AuthenticateAccessToken(ctx context.Context, token string, now time.Time) (*domain.Session, *domain.User, error) {
	b.logger.Info("dispatching query", "query", "authenticate_access_token")
	session, user, err := b.authenticate.Execute(ctx, token, now)
	if err != nil {
		b.logger.Warn("query failed", "query", "authenticate_access_token", "error", err)
	}
	return session, user, err
}

func (b *queryBus) ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error) {
	b.logger.Info("dispatching query", "query", "list_workspaces")
	workspaces, err := b.listWorkspaces.Execute(ctx, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_workspaces", "error", err)
	}
	return workspaces, err
}

func (b *queryBus) GetWorkspace(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error) {
	b.logger.Info("dispatching query", "query", "get_workspace")
	workspace, err := b.getWorkspace.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_workspace", "error", err)
	}
	return workspace, err
}

func (b *queryBus) ListWorkspaceMembers(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error) {
	b.logger.Info("dispatching query", "query", "list_workspace_members")
	members, err := b.listMembers.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_workspace_members", "error", err)
	}
	return members, err
}

func (b *queryBus) GetWorkspaceAccess(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	b.logger.Info("dispatching query", "query", "get_workspace_access")
	access, err := b.getAccess.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_workspace_access", "error", err)
	}
	return access, err
}

func (b *queryBus) ListWorkspaceInvitations(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	b.logger.Info("dispatching query", "query", "list_workspace_invitations")
	invitations, err := b.listInvites.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_workspace_invitations", "error", err)
	}
	return invitations, err
}
