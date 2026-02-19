package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// AuditAction defines known audit log actions.
const (
	AuditActionLogin         = "auth.login"
	AuditActionLoginFailed   = "auth.login_failed"
	AuditActionLogout        = "auth.logout"
	AuditActionTokenRefresh  = "auth.token_refresh"
	AuditActionCreateUser    = "admin.create_user"
	AuditActionUpdateRole    = "admin.update_role"
	AuditActionCreateTenant  = "admin.create_tenant"
	AuditActionDeleteTenant  = "admin.delete_tenant"
)

// AuditResult defines the result of an audited action.
const (
	AuditResultSuccess = "success"
	AuditResultFailure = "failure"
)

// AuditEntry represents a single audit log entry to be persisted.
type AuditEntry struct {
	TenantID     uuid.UUID
	UserID       uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
	Result       string
	Metadata     map[string]interface{}
	IPAddress    string
}

// AuditStore is the interface for persisting audit log entries.
type AuditStore interface {
	InsertAuditLog(ctx context.Context, entry AuditEntry) error
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
func (al *AuditLogger) LogAuthAttempt(ctx context.Context, r *http.Request, tenantID, userID uuid.UUID, action string) {
	entry := AuditEntry{
		TenantID:     tenantID,
		UserID:       userID,
		Action:       action,
		ResourceType: "session",
		Result:       AuditResultSuccess,
		IPAddress:    extractIP(r),
	}

	al.log(ctx, entry)
}

// LogAuthFailure logs a failed authentication event.
func (al *AuditLogger) LogAuthFailure(ctx context.Context, r *http.Request, action, reason string) {
	tenantID := TenantFromContext(ctx)
	userID := UserFromContext(ctx)

	entry := AuditEntry{
		TenantID:     tenantID,
		UserID:       userID,
		Action:       action,
		ResourceType: "session",
		Result:       AuditResultFailure,
		Metadata:     map[string]interface{}{"reason": reason},
		IPAddress:    extractIP(r),
	}

	al.log(ctx, entry)
}

// LogAdminAction logs an administrative action (user creation, role change, etc.).
func (al *AuditLogger) LogAdminAction(ctx context.Context, r *http.Request, action, resourceType, resourceID string, metadata map[string]interface{}) {
	tenantID := TenantFromContext(ctx)
	userID := UserFromContext(ctx)

	entry := AuditEntry{
		TenantID:     tenantID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       AuditResultSuccess,
		Metadata:     metadata,
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
		Str("result", entry.Result).
		Str("ip_address", entry.IPAddress)

	if entry.TenantID != uuid.Nil {
		event = event.Str("tenant_id", entry.TenantID.String())
	}
	if entry.UserID != uuid.Nil {
		event = event.Str("user_id", entry.UserID.String())
	}
	if entry.ResourceID != "" {
		event = event.Str("resource_id", entry.ResourceID)
	}

	event.Msg("audit log")

	// Persist to database
	if al.store != nil {
		if err := al.store.InsertAuditLog(ctx, entry); err != nil {
			al.logger.Error().Err(err).
				Str("action", entry.Action).
				Msg("failed to persist audit log")
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

// MetadataToJSON converts metadata map to JSONB-compatible bytes.
func MetadataToJSON(metadata map[string]interface{}) []byte {
	if metadata == nil {
		return nil
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil
	}
	return data
}

// IPToInet converts an IP address string to a pgtype.Text for storage in an INET column.
// PostgreSQL will implicitly cast the text value to INET.
func IPToInet(ipStr string) pgtype.Text {
	if ipStr == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: ipStr, Valid: true}
}
