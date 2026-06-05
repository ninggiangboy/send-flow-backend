package security

import "testing"

func TestPasswordHasher_HashAndCompare(t *testing.T) {
	hasher := NewPasswordHasher(4)
	hash, err := hasher.Hash("secret-password")
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	if err := hasher.Compare(hash, "secret-password"); err != nil {
		t.Fatalf("expected password to match: %v", err)
	}
	if err := hasher.Compare(hash, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestHMACSHA256_Verify(t *testing.T) {
	signature, err := SignHMACSHA256("payload", "secret")
	if err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}
	if !VerifyHMACSHA256("payload", signature, "secret") {
		t.Fatal("expected signature to verify")
	}
	if VerifyHMACSHA256("payload", signature, "other-secret") {
		t.Fatal("expected signature to fail with other secret")
	}
}
