package app

import (
	"context"
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
	signup        *signup.Handler
	login         *login.Handler
	oauthStart    *oauthstart.Handler
	oauthEx       *oauthexchange.Handler
	revoke        *revokesession.Handler
	refresh       *refresh.Handler
	reqVerify     *requestemailverification.Handler
	verify        *verifyemail.Handler
	forgot        *forgotpassword.Handler
	reset         *resetpassword.Handler
	mfaLogin      *mfalogin.Handler
	mfaSetup      *mfatotpsetup.Handler
	mfaEnable     *mfatotpenable.Handler
	mfaDisable    *mfatotpdisable.Handler
	mfaRegen      *mfaregenerate.Handler
	createWS      *createworkspace.Handler
	inviteMember  *inviteworkspacemember.Handler
	acceptInvite  *acceptworkspaceinvitation.Handler
	removeMember  *removeworkspacemember.Handler
	updateRole    *updateworkspacememberrole.Handler
}

func newCommandBus(
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
	return b.signup.Execute(ctx, cmd)
}
func (b *commandBus) Login(ctx context.Context, cmd login.Command) (*usecase.LoginResult, error) {
	return b.login.Execute(ctx, cmd)
}
func (b *commandBus) OAuthStart(ctx context.Context, cmd oauthstart.Command) (*usecase.OAuthStartResult, error) {
	return b.oauthStart.Execute(ctx, cmd)
}
func (b *commandBus) OAuthExchange(ctx context.Context, cmd oauthexchange.Command) (*usecase.SessionContext, *domain.OAuthIdentity, error) {
	return b.oauthEx.Execute(ctx, cmd)
}
func (b *commandBus) RevokeSession(ctx context.Context, sessionID, userID string, now time.Time) error {
	return b.revoke.Execute(ctx, sessionID, userID, now)
}
func (b *commandBus) Refresh(ctx context.Context, cmd refresh.Command) (*usecase.SessionContext, error) {
	return b.refresh.Execute(ctx, cmd)
}
func (b *commandBus) RequestEmailVerification(ctx context.Context, userID string, now time.Time) error {
	return b.reqVerify.Execute(ctx, userID, now)
}
func (b *commandBus) VerifyEmail(ctx context.Context, token string, now time.Time) error {
	return b.verify.Execute(ctx, token, now)
}
func (b *commandBus) ForgotPassword(ctx context.Context, email string, now time.Time) error {
	return b.forgot.Execute(ctx, email, now)
}
func (b *commandBus) ResetPassword(ctx context.Context, cmd resetpassword.Command) error {
	return b.reset.Execute(ctx, cmd)
}
func (b *commandBus) MFALogin(ctx context.Context, cmd mfalogin.Command) (*usecase.SessionContext, error) {
	return b.mfaLogin.Execute(ctx, cmd)
}
func (b *commandBus) MFATOTPSetup(ctx context.Context, userID string, now time.Time) (*mfatotpsetup.Result, error) {
	return b.mfaSetup.Execute(ctx, userID, now)
}
func (b *commandBus) MFATOTPEnable(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error) {
	return b.mfaEnable.Execute(ctx, userID, code, now)
}
func (b *commandBus) MFATOTPDisable(ctx context.Context, cmd mfatotpdisable.Command) error {
	return b.mfaDisable.Execute(ctx, cmd)
}
func (b *commandBus) MFARegenerate(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error) {
	return b.mfaRegen.Execute(ctx, userID, code, now)
}
func (b *commandBus) CreateWorkspace(ctx context.Context, cmd createworkspace.Command) (*domain.Workspace, error) {
	return b.createWS.Execute(ctx, cmd)
}
func (b *commandBus) InviteWorkspaceMember(ctx context.Context, cmd inviteworkspacemember.Command) (*inviteworkspacemember.Result, error) {
	return b.inviteMember.Execute(ctx, cmd)
}
func (b *commandBus) AcceptWorkspaceInvitation(ctx context.Context, cmd acceptworkspaceinvitation.Command) (*domain.Membership, error) {
	return b.acceptInvite.Execute(ctx, cmd)
}
func (b *commandBus) RemoveWorkspaceMember(ctx context.Context, cmd removeworkspacemember.Command) error {
	return b.removeMember.Execute(ctx, cmd)
}
func (b *commandBus) UpdateWorkspaceMemberRole(ctx context.Context, cmd updateworkspacememberrole.Command) error {
	return b.updateRole.Execute(ctx, cmd)
}

type queryBus struct {
	listProvider    *listproviders.Handler
	getMe           *getme.Handler
	listSessions    *listsessions.Handler
	authn           *authenticate.Handler
	listWS          *listworkspaces.Handler
	getWS           *getworkspace.Handler
	listWSMembers   *listworkspacemembers.Handler
	getWSAccess     *getworkspaceaccess.Handler
	listWSInvites   *listworkspaceinvitations.Handler
}

func newQueryBus(
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

func (b *queryBus) ListProviders() []usecase.Provider { return b.listProvider.Execute() }
func (b *queryBus) GetMe(ctx context.Context, userID string) (*domain.User, error) {
	return b.getMe.Execute(ctx, userID)
}
func (b *queryBus) ListSessions(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	return b.listSessions.Execute(ctx, userID, now)
}
func (b *queryBus) AuthenticateAccessToken(ctx context.Context, token string) (*domain.Session, *domain.User, error) {
	return b.authn.Execute(ctx, token)
}
func (b *queryBus) ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error) {
	return b.listWS.Execute(ctx, userID)
}
func (b *queryBus) GetWorkspace(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error) {
	return b.getWS.Execute(ctx, workspaceID, userID)
}
func (b *queryBus) ListWorkspaceMembers(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error) {
	return b.listWSMembers.Execute(ctx, workspaceID, userID)
}
func (b *queryBus) GetWorkspaceAccess(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	return b.getWSAccess.Execute(ctx, workspaceID, userID)
}
func (b *queryBus) ListWorkspaceInvitations(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	return b.listWSInvites.Execute(ctx, workspaceID, userID)
}
