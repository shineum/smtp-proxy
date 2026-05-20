package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"
)

const (
	azureADTokenURLFmt = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	defaultScope       = "https://graph.microsoft.com/.default"
	tokenExpiryBuffer  = 5 * time.Minute
)

// TokenManager handles OAuth2 client credentials flow for Microsoft Graph.
// It caches tokens and refreshes them when expired or about to expire.
type TokenManager struct {
	mu           sync.RWMutex
	tenantID     string
	clientID     string
	clientSecret string
	tokenURL     string
	client       HTTPClient

	accessToken string
	expiresAt   time.Time
}

// NewTokenManager creates a token manager for Azure AD client credentials flow.
func NewTokenManager(tenantID, clientID, clientSecret string, client HTTPClient) *TokenManager {
	return &TokenManager{
		tenantID:     tenantID,
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     fmt.Sprintf(azureADTokenURLFmt, tenantID),
		client:       client,
	}
}

// GetToken returns a valid access token, refreshing if expired or near expiry.
func (tm *TokenManager) GetToken() (string, error) {
	tm.mu.RLock()
	if tm.accessToken != "" && time.Now().Before(tm.expiresAt.Add(-tokenExpiryBuffer)) {
		token := tm.accessToken
		tm.mu.RUnlock()
		return token, nil
	}
	tm.mu.RUnlock()

	return tm.refreshToken()
}

// refreshToken acquires a new token from Azure AD.
func (tm *TokenManager) refreshToken() (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock.
	if tm.accessToken != "" && time.Now().Before(tm.expiresAt.Add(-tokenExpiryBuffer)) {
		return tm.accessToken, nil
	}

	form := url.Values{}
	form.Set("client_id", tm.clientID)
	form.Set("client_secret", tm.clientSecret)
	form.Set("scope", defaultScope)
	form.Set("grant_type", "client_credentials")

	resp, err := tm.client.Do(&HTTPRequest{
		Method: "POST",
		URL:    tm.tokenURL,
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: []byte(form.Encode()),
	})
	if err != nil {
		return "", fmt.Errorf("msgraph auth: token request: %w", err)
	}

	if resp.StatusCode != 200 {
		// REQ-QUEUE-N006: defense-in-depth — strip any client_secret value
		// from the response body before it lands in error chains / logs.
		// Azure AD does not currently echo the request body back, but a
		// misconfigured proxy or future Azure change could.
		return "", fmt.Errorf("msgraph auth: token request returned status %d: %s", resp.StatusCode, redactClientSecret(string(resp.Body)))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		return "", fmt.Errorf("msgraph auth: parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("msgraph auth: empty access token in response")
	}

	tm.accessToken = tokenResp.AccessToken
	tm.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tm.accessToken, nil
}

// InvalidateToken clears the cached token, forcing a refresh on next call.
func (tm *TokenManager) InvalidateToken() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.accessToken = ""
	tm.expiresAt = time.Time{}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// secretRedactionPattern matches common secret-bearing form/JSON fields and
// replaces the value with "[REDACTED]". Patterns covered:
//   - form-encoded:   client_secret=...&  or  client_secret=...$
//   - JSON:           "client_secret":"..." (with optional whitespace)
//   - URL query:      ?client_secret=...   (handled by form-encoded rule)
//
// The middle group allows an optional closing quote after the key name
// (JSON style) and an optional opening quote before the value, so a single
// pattern handles both form and JSON encodings.
var secretRedactionPattern = regexp.MustCompile(
	`(?i)(client_secret|password|access_token|refresh_token|api[_-]?key)` +
		`("?\s*[:=]\s*"?)([^"&\s,}]*)`,
)

// redactClientSecret returns s with values of known secret-bearing fields
// replaced by "[REDACTED]". It is safe to use in error messages, structured
// log fields, and stack traces. Intended for defensive use where the input
// is *not expected* to contain secrets but might if upstream behaviour
// changes; do not rely on it as the primary line of defence.
func redactClientSecret(s string) string {
	return secretRedactionPattern.ReplaceAllString(s, "$1$2[REDACTED]")
}
