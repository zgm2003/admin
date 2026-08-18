package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	first, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("bcrypt hashes must use independent random salts")
	}
	if err := VerifyPassword(first, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(first, "wrong"); err == nil {
		t.Fatal("wrong password matched")
	}
}

func TestValidatePasswordRejectsInvalidLength(t *testing.T) {
	if err := ValidatePassword("1234567"); err == nil {
		t.Fatal("seven-rune password was accepted")
	}
	if err := ValidatePassword(strings.Repeat("界", 25)); err == nil {
		t.Fatal("75-byte password was accepted")
	}
	if err := ValidatePassword(strings.Repeat("界", 24)); err != nil {
		t.Fatalf("72-byte password rejected: %v", err)
	}
	if err := ValidatePassword("        "); err != nil {
		t.Fatalf("password was trimmed or composition-restricted: %v", err)
	}
}
