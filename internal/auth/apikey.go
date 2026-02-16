package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const apiKeyBytes = 32

// GenerateAPIKey generates a cryptographically secure API key.
// The key is 32 random bytes, hex-encoded to 64 characters.
func GenerateAPIKey() (string, error) {
	b := make([]byte, apiKeyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate API key: %w", err)
	}
	return hex.EncodeToString(b), nil
}
