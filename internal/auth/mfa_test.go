package auth

import (
	"testing"
	"time"
)

func TestVerifyTOTP(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Date(2024, time.March, 20, 12, 0, 0, 0, time.UTC)
	code, err := totpCode(secret, now)
	if err != nil {
		t.Fatalf("totpCode error: %v", err)
	}
	ok, err := VerifyTOTP(secret, code, now)
	if err != nil {
		t.Fatalf("VerifyTOTP error: %v", err)
	}
	if !ok {
		t.Fatal("expected code to verify")
	}
}

func TestRecoveryCodeMatch(t *testing.T) {
	code := "ABCD3-456EF"
	hash, err := HashRecoveryCode(code)
	if err != nil {
		t.Fatalf("HashRecoveryCode error: %v", err)
	}
	if !MatchRecoveryCode(hash, "abcd3 456ef") {
		t.Fatal("expected recovery code to match normalized input")
	}
	if MatchRecoveryCode(hash, "invalid") {
		t.Fatal("expected recovery code mismatch")
	}
}
