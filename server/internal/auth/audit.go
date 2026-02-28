package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// AuditAction defines known audit log actions.
const (
	AuditActionLogin        = "auth.login"
	AuditActionLoginFailed  = "auth.login_failed"
	AuditActionLogout       = "auth.logout"
	AuditActionTokenRefresh = "auth.token_refresh"
	AuditActionCreateUser   = "admin.create_user"
	AuditActionUpdateRole   = "admin.update_role"
	AuditActionCreateGroup  = "admin.create_group"
	AuditActionDeleteGroup  = "admin.delete_group"
)

// AuditEntry represents a single activity log entry to be persisted.
type AuditEntry struct {
	GroupID      uuid.UUID
	UserID       uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
	Changes      map[string]interface{}
	Comment      string
	IPAddress    string
}

// AuditStore is the interface for persisting activity log entries.
type AuditStore interface {
	InsertActivityLog(ctx context.Context, entry AuditEntry) error
}

// AuditLogger logs security-relevant events to the audit_logs table and zerolog.
type AuditLogger struct {
	store  AuditStore
	logger zerolog.Logger
}

// NewAuditLogger creates a new AuditLogger with the given store and logger.
func NewAuditLogger(store AuditStore, logger zerolog.Logger) *AuditLogger {
	return &AuditLogger{
		store:  store,
		logger: logger,
	}
}

// LogAuthAttempt logs a successful authentication event.
func (al *AuditLogger) LogAuthAttempt(ctx context.Context, r *http.Request, groupID, userID uuid.UUID, action string) {
	entry := AuditEntry{
		GroupID:      groupID,
		UserID:       userID,
		Action:       action,
		ResourceType: "session",
		IPAddress:    extractIP(r),
	}

	al.log(ctx, entry)
}

// LogAuthFailure logs a failed authentication event.
func (al *AuditLogger) LogAuthFailure(ctx context.Context, r *http.Request, action, reason string) {
	groupID := GroupIDFromContext(ctx)
	userID := UserFromContext(ctx)

	entry := AuditEntry{
		GroupID:      groupID,
		UserID:       userID,
		Action:       action,
		ResourceType: "session",
		Comment:      reason,
		IPAddress:    extractIP(r),
	}

	al.log(ctx, entry)
}

// LogAdminAction logs an administrative action (user creation, role change, etc.).
func (al *AuditLogger) LogAdminAction(ctx context.Context, r *http.Request, action, resourceType, resourceID string, changes map[string]interface{}) {
	groupID := GroupIDFromContext(ctx)
	userID := UserFromContext(ctx)

	entry := AuditEntry{
		GroupID:      groupID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      changes,
		IPAddress:    extractIP(r),
	}

	al.log(ctx, entry)
}

// log persists the audit entry and logs it via zerolog.
func (al *AuditLogger) log(ctx context.Context, entry AuditEntry) {
	// Log to structured logger
	event := al.logger.Info().
		Str("action", entry.Action).
		Str("resource_type", entry.ResourceType).
		Str("ip_address", entry.IPAddress)

	if entry.GroupID != uuid.Nil {
		event = event.Str("group_id", entry.GroupID.String())
	}
	if entry.UserID != uuid.Nil {
		event = event.Str("user_id", entry.UserID.String())
	}
	if entry.ResourceID != "" {
		event = event.Str("resource_id", entry.ResourceID)
	}
	if entry.Comment != "" {
		event = event.Str("comment", entry.Comment)
	}

	event.Msg("activity log")

	// Persist to database
	if al.store != nil {
		if err := al.store.InsertActivityLog(ctx, entry); err != nil {
			al.logger.Error().Err(err).
				Str("action", entry.Action).
				Msg("failed to persist activity log")
		}
	}
}

// extractIP extracts the client IP address from the request,
// checking X-Forwarded-For and X-Real-IP headers first.
func extractIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the first IP in the chain
		if idx := len(xff); idx > 0 {
			for i, c := range xff {
				if c == ',' {
					return xff[:i]
				}
			}
			return xff
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ChangesToJSON converts a changes map to JSONB-compatible bytes.
func ChangesToJSON(changes map[string]interface{}) []byte {
	if changes == nil {
		return nil
	}
	data, err := json.Marshal(changes)
	if err != nil {
		return nil
	}
	return data
}

// IPToInet converts an IP address string to a *netip.Addr for storage in an INET column.
func IPToInet(ipStr string) *netip.Addr {
	if ipStr == "" {
		return nil
	}
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil
	}
	return &addr
}
