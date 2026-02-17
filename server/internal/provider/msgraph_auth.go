package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
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
		return "", fmt.Errorf("msgraph auth: token request returned status %d: %s", resp.StatusCode, string(resp.Body))
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
