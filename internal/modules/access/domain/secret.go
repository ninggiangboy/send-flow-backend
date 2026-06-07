package domain

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	SecretStablePrefix = "sk_"
	SecretKeyLen       = 32
	PrefixVisibleChars = 14
)

type SecretGenerator struct{}

func NewSecretGenerator() SecretGenerator {
	return SecretGenerator{}
}

func (g SecretGenerator) Generate() (plaintext string, keyPrefix string, err error) {
	buf := make([]byte, SecretKeyLen)
	if _, readErr := rand.Read(buf); readErr != nil {
		return "", "", fmt.Errorf("generate secret: %w", readErr)
	}
	encoded := base64.RawURLEncoding.EncodeToString(buf)
	plaintext = SecretStablePrefix + encoded
	keyPrefix = derivePrefix(plaintext)
	return plaintext, keyPrefix, nil
}

func derivePrefix(secret string) string {
	if len(secret) < PrefixVisibleChars {
		return secret
	}
	return secret[:PrefixVisibleChars]
}

type SecretHasher struct{}

func NewSecretHasher() SecretHasher {
	return SecretHasher{}
}

func (h SecretHasher) Hash(secret string) (string, error) {
	if secret == "" {
		return "", errors.New("secret is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash secret: %w", err)
	}
	return string(hash), nil
}

func (h SecretHasher) Verify(hash, secret string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret))
	return err == nil
}

func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func ExtractBearerToken(authHeader string) (string, bool) {
	h := strings.TrimSpace(authHeader)
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return "", false
	}
	token := strings.TrimSpace(h[len("Bearer "):])
	if token == "" {
		return "", false
	}
	return token, true
}

func DerivePrefix(secret string) string {
	return derivePrefix(secret)
}
