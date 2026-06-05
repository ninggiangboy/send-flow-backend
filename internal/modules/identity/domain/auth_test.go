package domain

import (
	"testing"
	"time"
)

func TestNewEmailAddressNormalizes(t *testing.T) {
	e, err := NewEmailAddress("  A@EXAMPLE.com  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.String() != "a@example.com" {
		t.Fatalf("expected a@example.com, got %s", e.String())
	}
}

func TestNewEmailAddressRejectsEmpty(t *testing.T) {
	_, err := NewEmailAddress("")
	if err != ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestNewEmailAddressRejectsInvalid(t *testing.T) {
	cases := []string{"not-an-email", "@", "user@", "@domain.com"}
	for _, c := range cases {
		_, err := NewEmailAddress(c)
		if err != ErrInvalidEmail {
			t.Fatalf("expected ErrInvalidEmail for %q, got %v", c, err)
		}
	}
}

func TestNewEmailAddressAcceptsValid(t *testing.T) {
	cases := []string{"a@example.com", "user+tag@domain.org", "first.last@company.co"}
	for _, c := range cases {
		e, err := NewEmailAddress(c)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", c, err)
		}
		if e.String() == "" {
			t.Fatalf("expected non-empty for %q", c)
		}
	}
}

func TestNewUser(t *testing.T) {
	email, _ := NewEmailAddress("user@example.com")
	now := time.Now().UTC()
	u := NewUser("id-1", email, "hashed-pw", "password", now)
	if u.ID != "id-1" {
		t.Fatalf("expected id-1, got %s", u.ID)
	}
	if u.Email != "user@example.com" {
		t.Fatalf("expected user@example.com, got %s", u.Email)
	}
	if u.HashedPassword != "hashed-pw" {
		t.Fatalf("expected hashed-pw, got %s", u.HashedPassword)
	}
	if u.PrimaryAuthMethod != "password" {
		t.Fatalf("expected password, got %s", u.PrimaryAuthMethod)
	}
	if u.CreatedAt != now || u.UpdatedAt != now {
		t.Fatalf("timestamps not set correctly")
	}
}

func TestNewOAuthUser(t *testing.T) {
	email, _ := NewEmailAddress("oauth@provider.com")
	now := time.Now().UTC()
	u := NewOAuthUser("id-2", email, "oauth_google", now)
	if u.ID != "id-2" {
		t.Fatalf("expected id-2, got %s", u.ID)
	}
	if u.Email != "oauth@provider.com" {
		t.Fatalf("expected oauth@provider.com, got %s", u.Email)
	}
	if u.HashedPassword != "" {
		t.Fatalf("expected empty hashed password for oauth user, got %s", u.HashedPassword)
	}
	if u.PrimaryAuthMethod != "oauth_google" {
		t.Fatalf("expected oauth_google, got %s", u.PrimaryAuthMethod)
	}
}

func TestUserVerifyPassword(t *testing.T) {
	email, _ := NewEmailAddress("user@example.com")
	now := time.Now().UTC()
	u := NewUser("id-1", email, "hashed-pw", "password", now)
	hasher := &stubHasher{compareErr: nil}
	if err := u.VerifyPassword("raw-pw", hasher); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	hasherFail := &stubHasher{compareErr: ErrInvalidCredentials}
	if err := u.VerifyPassword("wrong-pw", hasherFail); err == nil {
		t.Fatal("expected error for wrong password")
	}
}

type stubHasher struct {
	compareErr error
}

func (s *stubHasher) Hash(password string) (string, error) { return "hashed", nil }
func (s *stubHasher) Compare(hash, password string) error  { return s.compareErr }

func TestNewSession(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.Add(1 * time.Hour)
	s := NewSession("s-1", "u-1", "password", "a-jti", "r-jti", expiresAt, "127.0.0.1", "agent", now)
	if s.ID != "s-1" {
		t.Fatalf("expected s-1, got %s", s.ID)
	}
	if s.UserID != "u-1" {
		t.Fatalf("expected u-1, got %s", s.UserID)
	}
	if s.AuthMethod != "password" {
		t.Fatalf("expected password, got %s", s.AuthMethod)
	}
	if s.AccessJTI != "a-jti" || s.RefreshJTI != "r-jti" {
		t.Fatalf("jtis not set correctly")
	}
	if s.IPAddress != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %s", s.IPAddress)
	}
}

func TestSessionIsActive(t *testing.T) {
	now := time.Now().UTC()
	s := NewSession("s-1", "u-1", "password", "a-jti", "r-jti", now.Add(1*time.Hour), "ip", "ua", now)
	if !s.IsActive(now) {
		t.Fatal("expected session to be active")
	}
	if s.IsActive(now.Add(2 * time.Hour)) {
		t.Fatal("expected session to be inactive when expired")
	}
}

func TestSessionIsRevoked(t *testing.T) {
	now := time.Now().UTC()
	s := NewSession("s-1", "u-1", "password", "a-jti", "r-jti", now.Add(1*time.Hour), "ip", "ua", now)
	if s.IsRevoked() {
		t.Fatal("expected session to not be revoked")
	}
	revokedAt := now.Add(5 * time.Minute)
	s.RevokedAt = &revokedAt
	if !s.IsRevoked() {
		t.Fatal("expected session to be revoked")
	}
}

func TestSessionIsActiveWithRevoked(t *testing.T) {
	now := time.Now().UTC()
	s := NewSession("s-1", "u-1", "password", "a-jti", "r-jti", now.Add(1*time.Hour), "ip", "ua", now)
	revokedAt := now.Add(5 * time.Minute)
	s.RevokedAt = &revokedAt
	if s.IsActive(now) {
		t.Fatal("expected revoked session to not be active")
	}
}

func TestSessionIsExpired(t *testing.T) {
	now := time.Now().UTC()
	s := NewSession("s-1", "u-1", "password", "a-jti", "r-jti", now.Add(1*time.Hour), "ip", "ua", now)
	if s.IsExpired(now) {
		t.Fatal("expected session to not be expired")
	}
	if !s.IsExpired(now.Add(2 * time.Hour)) {
		t.Fatal("expected session to be expired")
	}
}

func TestNewExternalAuthAccount(t *testing.T) {
	now := time.Now().UTC()
	a := NewExternalAuthAccount("acc-1", "u-1", "google", "gid-123", "user@example.com", true, now)
	if a.ID != "acc-1" {
		t.Fatalf("expected acc-1, got %s", a.ID)
	}
	if a.UserID != "u-1" {
		t.Fatalf("expected u-1, got %s", a.UserID)
	}
	if a.Provider != "google" {
		t.Fatalf("expected google, got %s", a.Provider)
	}
	if a.ProviderUserID != "gid-123" {
		t.Fatalf("expected gid-123, got %s", a.ProviderUserID)
	}
	if a.ProviderEmail != "user@example.com" {
		t.Fatalf("expected user@example.com, got %s", a.ProviderEmail)
	}
	if !a.ProviderEmailVerified {
		t.Fatal("expected ProviderEmailVerified to be true")
	}
	if a.LinkedAt != now {
		t.Fatal("expected LinkedAt to be now")
	}
}
