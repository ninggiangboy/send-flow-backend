package constants

// Error code keys used in API responses via writeError() calls.
const (
	ErrorInternal                  = "internal.error"
	ErrorInvalidRequestBody        = "auth.invalid_request_body"
	ErrorInvalidToken              = "auth.invalid_token"
	ErrorPermissionDenied          = "auth.permission_denied"
	ErrorRateLimited               = "auth.rate_limited"
	ErrorAPIKeyInvalid             = "api_key.invalid"
	ErrorAPIKeyRevoked             = "api_key.revoked"
	ErrorDeliveryMessageNotFound   = "delivery.message_not_found"
	ErrorSenderDomainNotFound      = "sender.domain_not_found"
	ErrorTemplateNotFound          = "template.not_found"
	ErrorNotificationFilterInvalid = "notification.filter_invalid"
	ErrorMissingWorkspace          = "workspace.missing"
	ErrorWorkspaceNotFound         = "workspace.not_found"
)
