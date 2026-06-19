package domain

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
