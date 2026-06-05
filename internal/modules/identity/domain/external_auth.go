package domain

import "time"

type ExternalAuthAccount struct {
	ID                    string
	UserID                string
	Provider              string
	ProviderUserID        string
	ProviderEmail         string
	ProviderEmailVerified bool
	LinkedAt              time.Time
	LastLoginAt           *time.Time
}

func NewExternalAuthAccount(id, userID, provider, providerUserID, providerEmail string, providerEmailVerified bool, now time.Time) ExternalAuthAccount {
	return ExternalAuthAccount{
		ID:                    id,
		UserID:                userID,
		Provider:              provider,
		ProviderUserID:        providerUserID,
		ProviderEmail:         providerEmail,
		ProviderEmailVerified: providerEmailVerified,
		LinkedAt:              now,
	}
}

type OAuthIdentity struct {
	Provider       string
	ProviderUserID string
	Email          string
	EmailVerified  bool
}
