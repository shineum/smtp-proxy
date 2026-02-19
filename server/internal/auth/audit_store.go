package auth

import (
	"context"
)

// InsertAuditLogFunc is a function signature for persisting audit log entries.
// This is used to bridge the auth package to the storage package without
// creating a circular dependency.
type InsertAuditLogFunc func(ctx context.Context, entry AuditEntry) error

// funcAuditStore wraps an InsertAuditLogFunc to implement AuditStore.
type funcAuditStore struct {
	fn InsertAuditLogFunc
}

// NewFuncAuditStore creates an AuditStore from a function.
// The caller provides a function (typically a closure over storage.Queries)
// that converts an AuditEntry to the storage-layer parameters and persists it.
//
// Example usage in main.go or server setup:
//
//	store := auth.NewFuncAuditStore(func(ctx context.Context, entry auth.AuditEntry) error {
//		_, err := queries.CreateAuditLog(ctx, storage.CreateAuditLogParams{
//			TenantID:     entry.TenantID,
//			UserID:       pgtype.UUID{Bytes: entry.UserID, Valid: entry.UserID != uuid.Nil},
//			Action:       entry.Action,
//			ResourceType: entry.ResourceType,
//			ResourceID:   pgtype.Text{String: entry.ResourceID, Valid: entry.ResourceID != ""},
//			Result:       entry.Result,
//			Metadata:     metadataToJSON(entry.Metadata),
//			IPAddress:    ipToInet(entry.IPAddress),
//		})
//		return err
//	})
func NewFuncAuditStore(fn InsertAuditLogFunc) AuditStore {
	return &funcAuditStore{fn: fn}
}

// InsertAuditLog implements AuditStore by delegating to the wrapped function.
func (s *funcAuditStore) InsertAuditLog(ctx context.Context, entry AuditEntry) error {
	return s.fn(ctx, entry)
}
