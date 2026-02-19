package api

import (
	"testing"
)

func TestHashToken(t *testing.T) {
	token := "test-token-value"

	hash1 := hashToken(token)
	hash2 := hashToken(token)

	if hash1 == "" {
		t.Error("hashToken() returned empty string")
	}

	if hash1 != hash2 {
		t.Errorf("hashToken() not deterministic: %q != %q", hash1, hash2)
	}

	// Different tokens should produce different hashes
	hash3 := hashToken("different-token")
	if hash1 == hash3 {
		t.Error("hashToken() returned same hash for different tokens")
	}

	// Hash should be a 64-character hex string (SHA-256)
	if len(hash1) != 64 {
		t.Errorf("hashToken() length = %d, want 64", len(hash1))
	}
}
