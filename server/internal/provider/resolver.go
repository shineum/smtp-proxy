package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

const defaultCacheTTL = 5 * time.Minute

// cachedProvider holds a provider instance and its expiration time.
type cachedProvider struct {
	provider  Provider
	expiresAt time.Time
}

// ProviderResolver resolves the ESP provider for a given user by looking up
// the user's provider_id in the database. Results are cached with a
// configurable TTL. When no provider is configured, behavior depends on
// allowStdoutFallback: if true (dev mode), a stdout provider is returned;
// if false (production), an error is returned.
type ProviderResolver struct {
	queries storage.Querier
	log     zerolog.Logger
	client  HTTPClient

	mu       sync.RWMutex
	cache    map[uuid.UUID]*cachedProvider
	cacheTTL time.Duration

	stdout              Provider
	allowStdoutFallback bool
}

// ErrNoProvider is returned when no ESP provider is configured for a user.
var ErrNoProvider = fmt.Errorf("no ESP provider configured")

// NewResolver creates a ProviderResolver that looks up providers from the database.
// When stdoutFallback is true (dev/test), falls back to stdout if no provider is configured.
// When stdoutFallback is false (production), returns ErrNoProvider instead.
func NewResolver(queries storage.Querier, client HTTPClient, log zerolog.Logger, stdoutFallback bool) *ProviderResolver {
	return &ProviderResolver{
		queries:             queries,
		log:                 log,
		client:              client,
		cache:               make(map[uuid.UUID]*cachedProvider),
		cacheTTL:            defaultCacheTTL,
		stdout:              NewStdout(ProviderConfig{Type: "stdout"}),
		allowStdoutFallback: stdoutFallback,
	}
}

// ResolveByUserID returns the ESP provider for the given user ID by looking up
// the user's provider_id column. This is the primary resolution path for SMTP
// service accounts that have a direct provider mapping.
func (r *ProviderResolver) ResolveByUserID(ctx context.Context, userID uuid.UUID) (Provider, error) {
	// Check cache under read lock.
	r.mu.RLock()
	if cached, ok := r.cache[userID]; ok && time.Now().Before(cached.expiresAt) {
		p := cached.provider
		r.mu.RUnlock()
		return p, nil
	}
	r.mu.RUnlock()

	// Cache miss or expired: look up user's provider_id.
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", userID, err)
	}

	if !user.ProviderID.Valid {
		return r.handleNoProvider(userID, "user")
	}

	providerID := uuid.UUID(user.ProviderID.Bytes)
	return r.resolveProviderByID(ctx, userID, providerID)
}

// Resolve returns the ESP provider for the given group ID (legacy path).
// It picks the first enabled provider in the group.
func (r *ProviderResolver) Resolve(ctx context.Context, groupID uuid.UUID) (Provider, error) {
	r.mu.RLock()
	if cached, ok := r.cache[groupID]; ok && time.Now().Before(cached.expiresAt) {
		p := cached.provider
		r.mu.RUnlock()
		return p, nil
	}
	r.mu.RUnlock()

	providers, err := r.queries.ListProvidersByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("list providers for group %s: %w", groupID, err)
	}

	var espProvider *storage.EspProvider
	for i := range providers {
		if providers[i].Enabled {
			espProvider = &providers[i]
			break
		}
	}

	if espProvider == nil {
		return r.handleNoProvider(groupID, "group")
	}

	return r.buildAndCache(groupID, espProvider)
}

// handleNoProvider handles the case when no provider is found.
func (r *ProviderResolver) handleNoProvider(id uuid.UUID, entity string) (Provider, error) {
	if r.allowStdoutFallback {
		r.log.Warn().
			Stringer(entity+"_id", id).
			Msg("no ESP provider found, falling back to stdout (dev mode)")
		r.cacheProvider(id, r.stdout)
		return r.stdout, nil
	}
	r.log.Error().
		Stringer(entity+"_id", id).
		Msg("no ESP provider configured")
	return nil, fmt.Errorf("%w for %s %s", ErrNoProvider, entity, id)
}

// resolveProviderByID loads an ESP provider by its ID and builds the provider instance.
func (r *ProviderResolver) resolveProviderByID(ctx context.Context, cacheKey uuid.UUID, providerID uuid.UUID) (Provider, error) {
	espProvider, err := r.queries.GetProviderByID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("get provider %s: %w", providerID, err)
	}

	if !espProvider.Enabled {
		return r.handleNoProvider(cacheKey, "user")
	}

	return r.buildAndCache(cacheKey, &espProvider)
}

// buildAndCache converts an ESP provider to a Provider instance and caches it.
func (r *ProviderResolver) buildAndCache(cacheKey uuid.UUID, espProvider *storage.EspProvider) (Provider, error) {
	cfg, err := espToConfig(espProvider)
	if err != nil {
		return nil, fmt.Errorf("convert provider config for %q: %w", espProvider.Name, err)
	}

	p, err := NewProvider(cfg, r.client)
	if err != nil {
		return nil, fmt.Errorf("create provider %q: %w", espProvider.Name, err)
	}

	r.log.Debug().
		Stringer("provider_id", espProvider.ID).
		Str("provider", p.GetName()).
		Msg("resolved provider from database")

	r.cacheProvider(cacheKey, p)
	return p, nil
}

// cacheProvider stores a provider in the cache with the configured TTL.
func (r *ProviderResolver) cacheProvider(id uuid.UUID, p Provider) {
	r.mu.Lock()
	r.cache[id] = &cachedProvider{
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

// pgUUIDToUUID converts a pgtype.UUID to a google/uuid.UUID.
// Returns uuid.Nil if the pgtype.UUID is not valid.
func pgUUIDToUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}
