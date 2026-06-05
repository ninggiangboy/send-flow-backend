package domain

import "time"

type Session struct {
	ID         string
	UserID     string
	AuthMethod string
	AccessJTI  string
	RefreshJTI string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	IPAddress  string
	UserAgent  string
	CreatedAt  time.Time
}

func NewSession(id, userID, authMethod, accessJTI, refreshJTI string, expiresAt time.Time, ip, ua string, now time.Time) Session {
	return Session{
		ID:         id,
		UserID:     userID,
		AuthMethod: authMethod,
		AccessJTI:  accessJTI,
		RefreshJTI: refreshJTI,
		ExpiresAt:  expiresAt,
		IPAddress:  ip,
		UserAgent:  ua,
		CreatedAt:  now,
	}
}

func (s Session) IsActive(now time.Time) bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(now)
}

func (s Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

func (s Session) IsExpired(now time.Time) bool {
	return !s.ExpiresAt.After(now)
}
