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
)

type CommandBus interface {
	Signup(ctx context.Context, cmd signup.Command) (*usecase.SessionContext, error)
	Login(ctx context.Context, cmd login.Command) (*usecase.LoginResult, error)
	OAuthStart(ctx context.Context, cmd oauthstart.Command) (*usecase.OAuthStartResult, error)
	OAuthExchange(ctx context.Context, cmd oauthexchange.Command) (*usecase.SessionContext, *domain.OAuthIdentity, error)
	RevokeSession(ctx context.Context, sessionID, userID string, now time.Time) error
	Refresh(ctx context.Context, cmd refresh.Command) (*usecase.SessionContext, error)
	RequestEmailVerification(ctx context.Context, userID string, now time.Time) error
	VerifyEmail(ctx context.Context, token string, now time.Time) error
	ForgotPassword(ctx context.Context, email string, now time.Time) error
	ResetPassword(ctx context.Context, cmd resetpassword.Command) error
	MFALogin(ctx context.Context, cmd mfalogin.Command) (*usecase.SessionContext, error)
	MFATOTPSetup(ctx context.Context, userID string, now time.Time) (*mfatotpsetup.Result, error)
	MFATOTPEnable(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error)
	MFATOTPDisable(ctx context.Context, cmd mfatotpdisable.Command) error
	MFARegenerate(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error)
	CreateWorkspace(ctx context.Context, cmd createworkspace.Command) (*domain.Workspace, error)
	InviteWorkspaceMember(ctx context.Context, cmd inviteworkspacemember.Command) (*inviteworkspacemember.Result, error)
	AcceptWorkspaceInvitation(ctx context.Context, cmd acceptworkspaceinvitation.Command) (*domain.Membership, error)
	RemoveWorkspaceMember(ctx context.Context, cmd removeworkspacemember.Command) error
	UpdateWorkspaceMemberRole(ctx context.Context, cmd updateworkspacememberrole.Command) error
}

type QueryBus interface {
	ListProviders() []usecase.Provider
	GetMe(ctx context.Context, userID string) (*domain.User, error)
	ListSessions(ctx context.Context, userID string, now time.Time) ([]domain.Session, error)
	AuthenticateAccessToken(ctx context.Context, token string) (*domain.Session, *domain.User, error)
	ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error)
	ListWorkspaceMembers(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error)
	GetWorkspaceAccess(ctx context.Context, workspaceID, userID string) (*domain.Membership, error)
	ListWorkspaceInvitations(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error)
}

type commandBus struct {
	logger       *slog.Logger
	signup       *signup.Handler
	login        *login.Handler
	oauthStart   *oauthstart.Handler
	oauthEx      *oauthexchange.Handler
	revoke       *revokesession.Handler
	refresh      *refresh.Handler
	reqVerify    *requestemailverification.Handler
	verify       *verifyemail.Handler
	forgot       *forgotpassword.Handler
	reset        *resetpassword.Handler
	mfaLogin     *mfalogin.Handler
	mfaSetup     *mfatotpsetup.Handler
	mfaEnable    *mfatotpenable.Handler
	mfaDisable   *mfatotpdisable.Handler
	mfaRegen     *mfaregenerate.Handler
	createWS     *createworkspace.Handler
	inviteMember *inviteworkspacemember.Handler
	acceptInvite *acceptworkspaceinvitation.Handler
	removeMember *removeworkspacemember.Handler
	updateRole   *updateworkspacememberrole.Handler
}

func newCommandBus(
	logger *slog.Logger,
	signupH *signup.Handler,
	loginH *login.Handler,
	oauthStartH *oauthstart.Handler,
	oauthExchangeH *oauthexchange.Handler,
	revokeH *revokesession.Handler,
	refreshH *refresh.Handler,
	reqVerifyH *requestemailverification.Handler,
	verifyH *verifyemail.Handler,
	forgotH *forgotpassword.Handler,
	resetH *resetpassword.Handler,
	mfaLoginH *mfalogin.Handler,
	mfaSetupH *mfatotpsetup.Handler,
	mfaEnableH *mfatotpenable.Handler,
	mfaDisableH *mfatotpdisable.Handler,
	mfaRegenH *mfaregenerate.Handler,
	createWSH *createworkspace.Handler,
	inviteMemberH *inviteworkspacemember.Handler,
	acceptInviteH *acceptworkspaceinvitation.Handler,
	removeMemberH *removeworkspacemember.Handler,
	updateRoleH *updateworkspacememberrole.Handler,
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

func (b *commandBus) Signup(ctx context.Context, cmd signup.Command) (*usecase.SessionContext, error) {
	b.logger.Info("dispatching command", "command", "signup")
	result, err := b.signup.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "signup", "error", err)
	}
	return result, err
}
func (b *commandBus) Login(ctx context.Context, cmd login.Command) (*usecase.LoginResult, error) {
	b.logger.Info("dispatching command", "command", "login")
	result, err := b.login.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "login", "error", err)
	}
	return result, err
}
func (b *commandBus) OAuthStart(ctx context.Context, cmd oauthstart.Command) (*usecase.OAuthStartResult, error) {
	b.logger.Info("dispatching command", "command", "oauth_start")
	result, err := b.oauthStart.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "oauth_start", "error", err)
	}
	return result, err
}
func (b *commandBus) OAuthExchange(ctx context.Context, cmd oauthexchange.Command) (*usecase.SessionContext, *domain.OAuthIdentity, error) {
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
func (b *commandBus) Refresh(ctx context.Context, cmd refresh.Command) (*usecase.SessionContext, error) {
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
func (b *commandBus) ResetPassword(ctx context.Context, cmd resetpassword.Command) error {
	b.logger.Info("dispatching command", "command", "reset_password")
	err := b.reset.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "reset_password", "error", err)
	}
	return err
}
func (b *commandBus) MFALogin(ctx context.Context, cmd mfalogin.Command) (*usecase.SessionContext, error) {
	b.logger.Info("dispatching command", "command", "mfa_login")
	result, err := b.mfaLogin.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_login", "error", err)
	}
	return result, err
}
func (b *commandBus) MFATOTPSetup(ctx context.Context, userID string, now time.Time) (*mfatotpsetup.Result, error) {
	b.logger.Info("dispatching command", "command", "mfa_totp_setup")
	result, err := b.mfaSetup.Execute(ctx, userID, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_totp_setup", "error", err)
	}
	return result, err
}
func (b *commandBus) MFATOTPEnable(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error) {
	b.logger.Info("dispatching command", "command", "mfa_totp_enable")
	result, err := b.mfaEnable.Execute(ctx, userID, code, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_totp_enable", "error", err)
	}
	return result, err
}
func (b *commandBus) MFATOTPDisable(ctx context.Context, cmd mfatotpdisable.Command) error {
	b.logger.Info("dispatching command", "command", "mfa_totp_disable")
	err := b.mfaDisable.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_totp_disable", "error", err)
	}
	return err
}
func (b *commandBus) MFARegenerate(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error) {
	b.logger.Info("dispatching command", "command", "mfa_regenerate")
	result, err := b.mfaRegen.Execute(ctx, userID, code, now)
	if err != nil {
		b.logger.Warn("command failed", "command", "mfa_regenerate", "error", err)
	}
	return result, err
}
func (b *commandBus) CreateWorkspace(ctx context.Context, cmd createworkspace.Command) (*domain.Workspace, error) {
	b.logger.Info("dispatching command", "command", "create_workspace")
	result, err := b.createWS.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "create_workspace", "error", err)
	}
	return result, err
}
func (b *commandBus) InviteWorkspaceMember(ctx context.Context, cmd inviteworkspacemember.Command) (*inviteworkspacemember.Result, error) {
	b.logger.Info("dispatching command", "command", "invite_workspace_member")
	result, err := b.inviteMember.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "invite_workspace_member", "error", err)
	}
	return result, err
}
func (b *commandBus) AcceptWorkspaceInvitation(ctx context.Context, cmd acceptworkspaceinvitation.Command) (*domain.Membership, error) {
	b.logger.Info("dispatching command", "command", "accept_workspace_invitation")
	result, err := b.acceptInvite.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "accept_workspace_invitation", "error", err)
	}
	return result, err
}
func (b *commandBus) RemoveWorkspaceMember(ctx context.Context, cmd removeworkspacemember.Command) error {
	b.logger.Info("dispatching command", "command", "remove_workspace_member")
	err := b.removeMember.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "remove_workspace_member", "error", err)
	}
	return err
}
func (b *commandBus) UpdateWorkspaceMemberRole(ctx context.Context, cmd updateworkspacememberrole.Command) error {
	b.logger.Info("dispatching command", "command", "update_workspace_member_role")
	err := b.updateRole.Execute(ctx, cmd)
	if err != nil {
		b.logger.Warn("command failed", "command", "update_workspace_member_role", "error", err)
	}
	return err
}

type queryBus struct {
	logger        *slog.Logger
	listProvider  *listproviders.Handler
	getMe         *getme.Handler
	listSessions  *listsessions.Handler
	authn         *authenticate.Handler
	listWS        *listworkspaces.Handler
	getWS         *getworkspace.Handler
	listWSMembers *listworkspacemembers.Handler
	getWSAccess   *getworkspaceaccess.Handler
	listWSInvites *listworkspaceinvitations.Handler
}

func newQueryBus(
	logger *slog.Logger,
	listProviderH *listproviders.Handler,
	getMeH *getme.Handler,
	listSessionsH *listsessions.Handler,
	authnH *authenticate.Handler,
	listWSH *listworkspaces.Handler,
	getWSH *getworkspace.Handler,
	listWSMembersH *listworkspacemembers.Handler,
	getWSAccessH *getworkspaceaccess.Handler,
	listWSInvitesH *listworkspaceinvitations.Handler,
) QueryBus {
	return &queryBus{
		logger:        logger,
		listProvider:  listProviderH,
		getMe:         getMeH,
		listSessions:  listSessionsH,
		authn:         authnH,
		listWS:        listWSH,
		getWS:         getWSH,
		listWSMembers: listWSMembersH,
		getWSAccess:   getWSAccessH,
		listWSInvites: listWSInvitesH,
	}
}

func (b *queryBus) ListProviders() []usecase.Provider {
	b.logger.Info("dispatching query", "query", "list_providers")
	return b.listProvider.Execute()
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
func (b *queryBus) AuthenticateAccessToken(ctx context.Context, token string) (*domain.Session, *domain.User, error) {
	b.logger.Info("dispatching query", "query", "authenticate_access_token")
	session, user, err := b.authn.Execute(ctx, token)
	if err != nil {
		b.logger.Warn("query failed", "query", "authenticate_access_token", "error", err)
	}
	return session, user, err
}
func (b *queryBus) ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error) {
	b.logger.Info("dispatching query", "query", "list_workspaces")
	workspaces, err := b.listWS.Execute(ctx, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_workspaces", "error", err)
	}
	return workspaces, err
}
func (b *queryBus) GetWorkspace(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error) {
	b.logger.Info("dispatching query", "query", "get_workspace")
	workspace, err := b.getWS.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_workspace", "error", err)
	}
	return workspace, err
}
func (b *queryBus) ListWorkspaceMembers(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error) {
	b.logger.Info("dispatching query", "query", "list_workspace_members")
	members, err := b.listWSMembers.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_workspace_members", "error", err)
	}
	return members, err
}
func (b *queryBus) GetWorkspaceAccess(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	b.logger.Info("dispatching query", "query", "get_workspace_access")
	access, err := b.getWSAccess.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "get_workspace_access", "error", err)
	}
	return access, err
}
func (b *queryBus) ListWorkspaceInvitations(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	b.logger.Info("dispatching query", "query", "list_workspace_invitations")
	invitations, err := b.listWSInvites.Execute(ctx, workspaceID, userID)
	if err != nil {
		b.logger.Warn("query failed", "query", "list_workspace_invitations", "error", err)
	}
	return invitations, err
}
