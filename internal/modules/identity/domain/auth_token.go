package domain

import "time"

const (
	AuthTokenPurposeEmailVerification = "email_verification"
	AuthTokenPurposePasswordReset     = "password_reset"
	AuthTokenPurposeMFAChallenge      = "mfa_challenge"
)

type AuthToken struct {
	ID         string
	UserID     string
	Purpose    string
	TokenHash  string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

func (t AuthToken) IsUsable(now time.Time) bool {
	return t.ConsumedAt == nil && t.ExpiresAt.After(now)
}
