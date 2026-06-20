package app

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/membership"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfa"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/oauth"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/verification"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/workspace"
)

// ── Shared type aliases (moved from usecase/ to shared/) ──

type SessionContext = shared.SessionContext
type Provider = shared.Provider
type OAuthStartResult = shared.OAuthStartResult

// ── Command aliases ──

type SignupCommand = auth.SignupCommand
type LoginCommand = auth.LoginCommand
type RefreshCommand = auth.RefreshCommand
type MFALoginCommand = mfa.MFALoginCommand
type TOTPDisableCommand = mfa.TOTPDisableCommand
type ResetPasswordCommand = verification.ResetPasswordCommand
type CreateWorkspaceCommand = workspace.CreateCommand
type InviteWorkspaceMemberCommand = membership.InviteCommand
type AcceptWorkspaceInvitationCommand = membership.AcceptInvitationCommand
type RemoveWorkspaceMemberCommand = membership.RemoveCommand
type UpdateWorkspaceMemberRoleCommand = membership.UpdateRoleCommand
type OAuthStartCommand = oauth.StartCommand
type OAuthExchangeCommand = oauth.ExchangeCommand

// ── Result aliases ──

type TOTPSetupResult = mfa.TOTPSetupResult
type TOTPEnableResult = mfa.TOTPEnableResult
type InviteWorkspaceMemberResult = membership.InviteResult
