package constants

// Auth attempt metric result keys used in telemetry / metrics reporting.
const (
	AuthAttemptSignup       = "signup"
	AuthAttemptLogin        = "login"
	AuthAttemptMFA          = "mfa"
	AuthAttemptSuccess      = "success"
	AuthAttemptFailure      = "failure"
	AuthAttemptMissingToken = "missing_token"
	AuthAttemptRateLimited  = "rate_limited"
	AuthAttemptInvalid      = "invalid"
)
