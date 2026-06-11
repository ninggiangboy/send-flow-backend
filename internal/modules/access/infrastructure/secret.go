package infrastructure

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
)

type BcryptSecretHasher struct{}

func NewBcryptSecretHasher() *BcryptSecretHasher { return &BcryptSecretHasher{} }

func (h BcryptSecretHasher) Hash(secret string) (string, error) {
	if secret == "" {
		return "", errors.New("secret is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash secret: %w", err)
	}
	return string(hash), nil
}

func (h BcryptSecretHasher) Verify(hash, secret string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

type CryptoSecretGenerator struct{}

func NewCryptoSecretGenerator() *CryptoSecretGenerator { return &CryptoSecretGenerator{} }

func (g CryptoSecretGenerator) Generate() (plaintext, keyPrefix string, err error) {
	buf := make([]byte, domain.SecretKeyLen)
	if _, readErr := rand.Read(buf); readErr != nil {
		return "", "", fmt.Errorf("generate secret: %w", readErr)
	}
	encoded := base64.RawURLEncoding.EncodeToString(buf)
	plaintext = domain.SecretStablePrefix + encoded
	keyPrefix = domain.DerivePrefix(plaintext)
	return plaintext, keyPrefix, nil
}

var _ ports.SecretHasher = (*BcryptSecretHasher)(nil)
var _ ports.SecretGenerator = (*CryptoSecretGenerator)(nil)
