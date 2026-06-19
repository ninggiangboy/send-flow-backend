package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher struct {
	cost int
}

func NewPasswordHasher(cost int) PasswordHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return PasswordHasher{cost: cost}
}

func (h PasswordHasher) Hash(password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (h PasswordHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func RandomToken(bytes int) (string, error) {
	if bytes <= 0 {
		return "", errors.New("token byte length must be positive")
	}
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func SignHMACSHA256(payload, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("secret is required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyHMACSHA256(payload, signature, secret string) bool {
	expected, err := SignHMACSHA256(payload, secret)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(strings.TrimSpace(signature)), []byte(expected))
}

func ValidatePasswordPolicy(password string) error {
	if len(password) > 128 {
		return errors.New("password must not exceed 128 characters")
	}
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return errors.New("password must include upper, lower, number, and special characters")
	}
	return nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

func GenerateRecoveryCode() (string, error) {
	raw, err := RandomToken(12)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(raw[:4] + "-" + raw[4:8] + "-" + raw[8:12]), nil
}

// TOTPCode computes a time-based one-time password per RFC 6238.
// SHA-1 is specified by RFC 6238 and is the most widely compatible TOTP hash function.
func TOTPCode(secret string, at time.Time, period uint) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("decode totp secret: %w", err)
	}
	if period == 0 {
		period = 30
	}
	counter := uint64(at.UTC().Unix() / int64(period))
	msg := []byte{
		byte(counter >> 56), byte(counter >> 48), byte(counter >> 40), byte(counter >> 32),
		byte(counter >> 24), byte(counter >> 16), byte(counter >> 8), byte(counter),
	}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[offset]&0x7f) << 24) | (uint32(sum[offset+1]) << 16) | (uint32(sum[offset+2]) << 8) | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", bin%1000000), nil
}

func VerifyTOTPCode(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	for _, skew := range []time.Duration{-30 * time.Second, 0, 30 * time.Second} {
		expected, err := TOTPCode(secret, now.Add(skew), 30)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}
