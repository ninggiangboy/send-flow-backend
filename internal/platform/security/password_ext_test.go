package security

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func knownTOTPSecret(t *testing.T) string {
	t.Helper()
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

func TestPasswordHasher_Hash_EmptyPassword(t *testing.T) {
	hasher := NewPasswordHasher(bcrypt.DefaultCost)
	_, err := hasher.Hash("")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
	if err.Error() != "password is required" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestNewPasswordHasher_ZeroCost(t *testing.T) {
	h := NewPasswordHasher(0)
	if h.cost != bcrypt.DefaultCost {
		t.Fatalf("expected cost %d, got %d", bcrypt.DefaultCost, h.cost)
	}
	h2 := NewPasswordHasher(10)
	if h2.cost != 10 {
		t.Fatalf("expected cost 10, got %d", h2.cost)
	}
}

func TestRandomToken_ZeroBytes(t *testing.T) {
	_, err := RandomToken(0)
	if err == nil {
		t.Fatal("expected error for zero bytes")
	}
}

func TestRandomToken_NegativeBytes(t *testing.T) {
	_, err := RandomToken(-1)
	if err == nil {
		t.Fatal("expected error for negative bytes")
	}
}

func TestRandomToken_PositiveBytes(t *testing.T) {
	token, err := RandomToken(16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	const expectedLen = 22
	if len(token) != expectedLen {
		t.Fatalf("expected length %d, got %d", expectedLen, len(token))
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "too short",
			password: "Abc1!",
			wantErr:  true,
		},
		{
			name:     "missing uppercase",
			password: "abcdef1!@#$%^",
			wantErr:  true,
		},
		{
			name:     "missing lowercase",
			password: "ABCDEF1!@#$%^",
			wantErr:  true,
		},
		{
			name:     "missing digit",
			password: "Abcdefgh!@#$%",
			wantErr:  true,
		},
		{
			name:     "missing special character",
			password: "Abcdefgh123456",
			wantErr:  true,
		},
		{
			name:     "valid password",
			password: "ValidP@ssw0rd!",
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordPolicy(tt.password)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHashToken_Consistent(t *testing.T) {
	token := "my-secret-token-123"
	h1 := HashToken(token)
	h2 := HashToken(token)
	if h1 != h2 {
		t.Fatal("expected same token to produce identical hash")
	}
}

func TestHashToken_DifferentTokens(t *testing.T) {
	h1 := HashToken("token-a")
	h2 := HashToken("token-b")
	if h1 == h2 {
		t.Fatal("expected different tokens to produce different hashes")
	}
}

func TestGenerateTOTPSecret_Length(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const expectedLen = 32
	if len(secret) != expectedLen {
		t.Fatalf("expected length %d, got %d", expectedLen, len(secret))
	}
}

func TestGenerateRecoveryCode_Format(t *testing.T) {
	code, err := GenerateRecoveryCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != 14 {
		t.Fatalf("expected length 14, got %d", len(code))
	}
	if code[4] != '-' || code[9] != '-' {
		t.Fatalf("expected format XXXX-XXXX-XXXX, got %s", code)
	}
	if strings.ToUpper(code) != code {
		t.Fatal("expected recovery code to be uppercase")
	}
}

func TestTOTPCode_PeriodZeroDefaults(t *testing.T) {
	secret := knownTOTPSecret(t)
	now := time.Now()
	codeZero, err := TOTPCode(secret, now, 0)
	if err != nil {
		t.Fatalf("unexpected error with period 0: %v", err)
	}
	codeThirty, err := TOTPCode(secret, now, 30)
	if err != nil {
		t.Fatalf("unexpected error with period 30: %v", err)
	}
	if codeZero != codeThirty {
		t.Fatal("expected period 0 to default to period 30")
	}
}

func TestTOTPCode_InvalidBase32Secret(t *testing.T) {
	_, err := TOTPCode("!!invalid!!", time.Now(), 30)
	if err == nil {
		t.Fatal("expected error for invalid base32 secret")
	}
}

func TestTOTPCode_Returns6DigitCode(t *testing.T) {
	secret := knownTOTPSecret(t)
	code, err := TOTPCode(secret, time.Now(), 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6 digits, got %q (len=%d)", code, len(code))
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("expected only digits, got %q", code)
		}
	}
}

func TestVerifyTOTPCode_ValidCode(t *testing.T) {
	secret := knownTOTPSecret(t)
	now := time.Now()
	code, err := TOTPCode(secret, now, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !VerifyTOTPCode(secret, code, now) {
		t.Fatal("expected valid code to verify")
	}
}

func TestVerifyTOTPCode_InvalidCode(t *testing.T) {
	secret := knownTOTPSecret(t)
	if VerifyTOTPCode(secret, "000000", time.Now()) {
		t.Fatal("expected invalid code to fail")
	}
}

func TestVerifyTOTPCode_SkewTolerance(t *testing.T) {
	secret := knownTOTPSecret(t)
	now := time.Now()

	code, err := TOTPCode(secret, now, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !VerifyTOTPCode(secret, code, now.Add(-30*time.Second)) {
		t.Fatal("expected code to verify with -30s skew")
	}
	if !VerifyTOTPCode(secret, code, now.Add(30*time.Second)) {
		t.Fatal("expected code to verify with +30s skew")
	}

	if VerifyTOTPCode(secret, code, now.Add(-61*time.Second)) {
		t.Fatal("expected code to fail with -61s skew")
	}
	if VerifyTOTPCode(secret, code, now.Add(61*time.Second)) {
		t.Fatal("expected code to fail with +61s skew")
	}
}

func TestVerifyTOTPCode_WhitespaceTrimming(t *testing.T) {
	secret := knownTOTPSecret(t)
	now := time.Now()
	code, err := TOTPCode(secret, now, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !VerifyTOTPCode(secret, "  "+code+"  ", now) {
		t.Fatal("expected code with surrounding whitespace to verify")
	}
}
