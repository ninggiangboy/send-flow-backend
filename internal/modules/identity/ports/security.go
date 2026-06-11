package ports

import "time"

type TokenGenerator interface {
	RandomToken(n int) (string, error)
}

type TokenHasher interface {
	HashToken(token string) string
}

type PasswordValidator interface {
	Validate(password string) error
}

type TOTPCodeVerifier interface {
	VerifyTOTPCode(secret, code string, now time.Time) bool
}

type TOTPSecretGenerator interface {
	GenerateTOTPSecret() (string, error)
}

type RecoveryCodeGenerator interface {
	GenerateRecoveryCode() (string, error)
}
