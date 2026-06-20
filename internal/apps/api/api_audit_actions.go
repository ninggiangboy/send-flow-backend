package api

const (
	auditActionTransactionalSendAccepted = "transactional.send_accepted"

	auditActionWebhookCreated       = "webhook.created"
	auditActionWebhookUpdated       = "webhook.updated"
	auditActionWebhookDisabled      = "webhook.disabled"
	auditActionWebhookSecretRotated = "webhook.secret_rotated"

	auditActionWorkspaceInvitationCreated   = "workspace.invitation_created"
	auditActionWorkspaceInvitationAccepted  = "workspace.invitation_accepted"
	auditActionWorkspaceMemberRemoved       = "workspace.member_removed"
	auditActionWorkspaceMemberRoleUpdated   = "workspace.member_role_updated"
	auditActionWorkspaceMemberRolesAssigned = "workspace.member_roles_assigned"
	auditActionWorkspaceRoleCreated         = "workspace.role_created"
	auditActionWorkspaceRoleUpdated         = "workspace.role_updated"

	auditActionAPIKeyCreated = "api_key.created"
	auditActionAPIKeyUpdated = "api_key.updated"
	auditActionAPIKeyRotated = "api_key.rotated"
	auditActionAPIKeyRevoked = "api_key.revoked"

	auditActionSenderDomainCreated               = "sender_domain.created"
	auditActionSenderDomainVerificationRefreshed = "sender_domain.verification_refreshed"
	auditActionSenderDomainDisabled              = "sender_domain.disabled"

	auditActionAuthSignup                 = "auth.signup"
	auditActionAuthLoginMFARequired       = "auth.login_mfa_required"
	auditActionAuthLoginSuccess           = "auth.login_success"
	auditActionAuthMFALoginSuccess        = "auth.mfa_login_success"
	auditActionAuthLogout                 = "auth.logout"
	auditActionAuthVerificationEmailSent  = "auth.verification_email_sent"
	auditActionAuthEmailVerified          = "auth.email_verified"
	auditActionAuthPasswordReset          = "auth.password_reset"
	auditActionAuthMFASetup               = "auth.mfa_setup"
	auditActionAuthMFAEnabled             = "auth.mfa_enabled"
	auditActionAuthMFADisabled            = "auth.mfa_disabled"
	auditActionAuthMFARecoveryRegenerated = "auth.mfa_recovery_regenerated"
)
