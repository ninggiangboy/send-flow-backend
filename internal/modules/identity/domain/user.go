package domain

import "time"

type User struct {
	ID                string
	Email             string
	HashedPassword    string `json:"-"`
	PrimaryAuthMethod string
	EmailVerifiedAt   *time.Time
	MFAEnabledAt      *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewUser(id string, email EmailAddress, hashedPassword, authMethod string, now time.Time) User {
	return User{
		ID:                id,
		Email:             email.String(),
		HashedPassword:    hashedPassword,
		PrimaryAuthMethod: authMethod,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func NewOAuthUser(id string, email EmailAddress, authMethod string, now time.Time) User {
	user := User{
		ID:                id,
		Email:             email.String(),
		PrimaryAuthMethod: authMethod,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	user.EmailVerifiedAt = &now
	return user
}

func (u User) VerifyPassword(rawPassword string, hasher PasswordHasher) error {
	return hasher.Compare(u.HashedPassword, rawPassword)
}

func (u User) EmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

func (u User) MFAEnabled() bool {
	return u.MFAEnabledAt != nil
}
