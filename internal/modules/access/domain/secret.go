package domain

import (
	"crypto/subtle"
)

const (
	SecretStablePrefix = "sk_"
	SecretKeyLen       = 32
	PrefixVisibleChars = 14
)

func DerivePrefix(secret string) string {
	if len(secret) < PrefixVisibleChars {
		return secret
	}
	return secret[:PrefixVisibleChars]
}

func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
