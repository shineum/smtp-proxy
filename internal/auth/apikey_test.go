package auth

import (
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}

	if len(key) != 64 {
		t.Errorf("GenerateAPIKey() key length = %d, want 64", len(key))
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	key1, _ := GenerateAPIKey()
	key2, _ := GenerateAPIKey()

	if key1 == key2 {
		t.Error("GenerateAPIKey() produced duplicate keys")
	}
}

func TestGenerateAPIKey_HexEncoded(t *testing.T) {
	key, _ := GenerateAPIKey()

	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateAPIKey() contains non-hex character: %c", c)
			break
		}
	}
}
