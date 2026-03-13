package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	apiKeyBytes       = 32
	apiKeyPrefixLen   = 12
	apiKeyBcryptCost  = 12
)

// GenerateAPIKey generates a cryptographically secure API key.
// The key is 32 random bytes, hex-encoded to 64 characters.
func GenerateAPIKey() (string, error) {
	b := make([]byte, apiKeyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate API key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// APIKeyPrefix returns the first 12 characters of the plaintext key,
// used as a lookup prefix in the api_keys table.
func APIKeyPrefix(key string) string {
	if len(key) < apiKeyPrefixLen {
		return key
	}
	return key[:apiKeyPrefixLen]
}

// HashAPIKey returns a bcrypt hash of the full plaintext key.
func HashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), apiKeyBcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash API key: %w", err)
	}
	return string(hash), nil
}

// VerifyAPIKey reports whether the given plaintext key matches the stored hash.
func VerifyAPIKey(hash, key string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)) == nil
}
