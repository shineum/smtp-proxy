package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash == "" {
		t.Fatal("HashPassword() returned empty hash")
	}

	// bcrypt hashes start with $2a$ or $2b$
	if hash[0] != '$' {
		t.Errorf("HashPassword() hash does not start with $, got %q", hash[:4])
	}
}

func TestVerifyPassword_Correct(t *testing.T) {
	hash, err := HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if err := VerifyPassword(hash, "correctpassword"); err != nil {
		t.Errorf("VerifyPassword() with correct password returned error: %v", err)
	}
}

func TestVerifyPassword_Incorrect(t *testing.T) {
	hash, err := HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if err := VerifyPassword(hash, "wrongpassword"); err == nil {
		t.Error("VerifyPassword() with wrong password returned nil error")
	}
}

func TestHashPassword_DifferentHashesForSameInput(t *testing.T) {
	hash1, _ := HashPassword("samepassword")
	hash2, _ := HashPassword("samepassword")

	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes for same input (bcrypt should use random salt)")
	}
}
