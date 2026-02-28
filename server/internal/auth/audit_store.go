package auth

import (
	"context"
)

// InsertActivityLogFunc is a function signature for persisting activity log entries.
// This is used to bridge the auth package to the storage package without
// creating a circular dependency.
type InsertActivityLogFunc func(ctx context.Context, entry AuditEntry) error

// funcAuditStore wraps an InsertActivityLogFunc to implement AuditStore.
type funcAuditStore struct {
	fn InsertActivityLogFunc
}

// NewFuncAuditStore creates an AuditStore from a function.
// The caller provides a function (typically a closure over storage.Queries)
// that converts an AuditEntry to the storage-layer parameters and persists it.
//
// Example usage in main.go or server setup:
//
//	store := auth.NewFuncAuditStore(func(ctx context.Context, entry auth.AuditEntry) error {
//		_, err := queries.CreateActivityLog(ctx, storage.CreateActivityLogParams{
//			GroupID:      entry.GroupID,
//			ActorID:      entry.UserID,
//			Action:       entry.Action,
//			ResourceType: entry.ResourceType,
//			ResourceID:   pgtype.Text{String: entry.ResourceID, Valid: entry.ResourceID != ""},
//			Changes:      changesToJSON(entry.Changes),
//			Comment:      pgtype.Text{String: entry.Comment, Valid: entry.Comment != ""},
//			IpAddress:    ipToInet(entry.IPAddress),
//		})
//		return err
//	})
func NewFuncAuditStore(fn InsertActivityLogFunc) AuditStore {
	return &funcAuditStore{fn: fn}
}

// InsertActivityLog implements AuditStore by delegating to the wrapped function.
func (s *funcAuditStore) InsertActivityLog(ctx context.Context, entry AuditEntry) error {
	return s.fn(ctx, entry)
}
