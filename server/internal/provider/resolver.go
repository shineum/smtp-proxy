package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

const defaultCacheTTL = 5 * time.Minute

// cachedProvider holds a provider instance and its expiration time.
type cachedProvider struct {
	provider  Provider
	expiresAt time.Time
}

// ProviderResolver resolves the ESP provider for a given account by looking up
// the account's provider configuration in the database. Results are cached with
// a configurable TTL. When no provider is configured for an account, a shared
// stdout provider is returned as the default.
type ProviderResolver struct {
	queries storage.Querier
	log     zerolog.Logger
	client  HTTPClient

	mu       sync.RWMutex
	cache    map[uuid.UUID]*cachedProvider
	cacheTTL time.Duration

	stdout Provider
}

// NewResolver creates a ProviderResolver that looks up providers from the database
// and falls back to stdout when no provider is configured.
func NewResolver(queries storage.Querier, client HTTPClient, log zerolog.Logger) *ProviderResolver {
	return &ProviderResolver{
		queries:  queries,
		log:      log,
		client:   client,
		cache:    make(map[uuid.UUID]*cachedProvider),
		cacheTTL: defaultCacheTTL,
		stdout:   NewStdout(ProviderConfig{Type: "stdout"}),
	}
}

// Resolve returns the ESP provider for the given account ID.
// It checks the cache first, then queries the database. If no enabled provider
// is found, it returns the shared stdout provider.
func (r *ProviderResolver) Resolve(ctx context.Context, accountID uuid.UUID) (Provider, error) {
	// Check cache under read lock.
	r.mu.RLock()
	if cached, ok := r.cache[accountID]; ok && time.Now().Before(cached.expiresAt) {
		p := cached.provider
		r.mu.RUnlock()
		return p, nil
	}
	r.mu.RUnlock()

	// Cache miss or expired: query the database.
	providers, err := r.queries.ListProvidersByAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list providers for account %s: %w", accountID, err)
	}

	// Find the first enabled provider (ordered by created_at DESC from query).
	var espProvider *storage.EspProvider
	for i := range providers {
		if providers[i].Enabled {
			espProvider = &providers[i]
			break
		}
	}

	// No enabled provider found: return stdout default.
	if espProvider == nil {
		r.log.Debug().
			Stringer("account_id", accountID).
			Msg("no enabled provider found, using stdout default")
		r.cacheProvider(accountID, r.stdout)
		return r.stdout, nil
	}

	// Convert DB model to provider config and create the provider instance.
	cfg, err := espToConfig(espProvider)
	if err != nil {
		return nil, fmt.Errorf("convert provider config for %q: %w", espProvider.Name, err)
	}

	p, err := NewProvider(cfg, r.client)
	if err != nil {
		return nil, fmt.Errorf("create provider %q: %w", espProvider.Name, err)
	}

	r.log.Debug().
		Stringer("account_id", accountID).
		Str("provider", p.GetName()).
		Msg("resolved provider from database")

	r.cacheProvider(accountID, p)
	return p, nil
}

// cacheProvider stores a provider in the cache with the configured TTL.
func (r *ProviderResolver) cacheProvider(accountID uuid.UUID, p Provider) {
	r.mu.Lock()
	r.cache[accountID] = &cachedProvider{
		provider:  p,
		expiresAt: time.Now().Add(r.cacheTTL),
	}
	r.mu.Unlock()
}

// smtpConfigExtra holds optional fields parsed from the esp_providers.smtp_config JSONB column.
type smtpConfigExtra struct {
	Region       string `json:"region,omitempty"`
	Domain       string `json:"domain,omitempty"`
	TenantID     string `json:"tenant_id,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
}

// espToConfig converts a storage.EspProvider to a provider.ProviderConfig.
func espToConfig(esp *storage.EspProvider) (ProviderConfig, error) {
	cfg := ProviderConfig{
		Type: string(esp.ProviderType),
	}

	if esp.ApiKey.Valid {
		cfg.APIKey = esp.ApiKey.String
	}

	// Parse optional fields from smtp_config JSONB.
	if len(esp.SmtpConfig) > 0 {
		var extra smtpConfigExtra
		if err := json.Unmarshal(esp.SmtpConfig, &extra); err != nil {
			return cfg, fmt.Errorf("unmarshal smtp_config: %w", err)
		}
		cfg.Region = extra.Region
		cfg.Domain = extra.Domain
		cfg.TenantID = extra.TenantID
		cfg.ClientID = extra.ClientID
		cfg.ClientSecret = extra.ClientSecret
		cfg.UserID = extra.UserID
		if extra.Endpoint != "" {
			cfg.Endpoint = extra.Endpoint
		}
	}

	return cfg, nil
}
