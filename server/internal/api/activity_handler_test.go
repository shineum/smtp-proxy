package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestListActivityLogsHandler_Success(t *testing.T) {
	grp := testGroup()
	log := storage.ActivityLog{
		ID:           uuid.New(),
		GroupID:      grp.ID,
		ActorID:      pgtype.UUID{Bytes: testUser().ID, Valid: true},
		Action:       "admin.create_user",
		ResourceType: "user",
		ResourceID:   pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Changes:      []byte(`{"email":"new@test.com"}`),
		CreatedAt:    pgtype.Timestamptz{Valid: true},
	}

	mock := &mockQuerier{
		listActivityLogsByGroupIDFn: func(ctx context.Context, arg storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error) {
			if arg.GroupID != grp.ID {
				t.Errorf("expected group ID %s, got %s", grp.ID, arg.GroupID)
			}
			if arg.Limit != 50 {
				t.Errorf("expected default limit 50, got %d", arg.Limit)
			}
			if arg.Offset != 0 {
				t.Errorf("expected default offset 0, got %d", arg.Offset)
			}
			return []storage.ActivityLog{log}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String()+"/activity", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = setJWTContext(ctx, testUser().ID, grp.ID, "admin", "organization")
	req = req.WithContext(ctx)

	handler := ListActivityLogsHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp []activityLogResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 activity log, got %d", len(resp))
	}
	if resp[0].Action != "admin.create_user" {
		t.Errorf("expected action admin.create_user, got %s", resp[0].Action)
	}
}

func TestListActivityLogsHandler_WithPagination(t *testing.T) {
	grp := testGroup()

	mock := &mockQuerier{
		listActivityLogsByGroupIDFn: func(ctx context.Context, arg storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error) {
			if arg.Limit != 10 {
				t.Errorf("expected limit 10, got %d", arg.Limit)
			}
			if arg.Offset != 20 {
				t.Errorf("expected offset 20, got %d", arg.Offset)
			}
			return []storage.ActivityLog{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String()+"/activity?limit=10&offset=20", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = setJWTContext(ctx, testUser().ID, grp.ID, "admin", "organization")
	req = req.WithContext(ctx)

	handler := ListActivityLogsHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestListActivityLogsHandler_Forbidden(t *testing.T) {
	grp := testGroup()
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String()+"/activity", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	// Different group, not system type
	otherGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	ctx = setJWTContext(ctx, testUser().ID, otherGroupID, "admin", "company")
	req = req.WithContext(ctx)

	handler := ListActivityLogsHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestListActivityLogsHandler_SystemAdminCanAccessAnyGroup(t *testing.T) {
	grp := testGroup()
	mock := &mockQuerier{
		listActivityLogsByGroupIDFn: func(ctx context.Context, arg storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error) {
			return []storage.ActivityLog{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String()+"/activity", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	systemGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	ctx = setJWTContext(ctx, testUser().ID, systemGroupID, "admin", "system")
	req = req.WithContext(ctx)

	handler := ListActivityLogsHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestListActivityLogsHandler_InvalidGroupID(t *testing.T) {
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/bad-id/activity", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad-id")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = setJWTContext(ctx, testUser().ID, testGroup().ID, "admin", "organization")
	req = req.WithContext(ctx)

	handler := ListActivityLogsHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestListActivityLogsHandler_LimitCap(t *testing.T) {
	grp := testGroup()

	mock := &mockQuerier{
		listActivityLogsByGroupIDFn: func(ctx context.Context, arg storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error) {
			if arg.Limit != 100 {
				t.Errorf("expected limit capped at 100, got %d", arg.Limit)
			}
			return []storage.ActivityLog{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String()+"/activity?limit=999", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = setJWTContext(ctx, testUser().ID, grp.ID, "admin", "organization")
	req = req.WithContext(ctx)

	handler := ListActivityLogsHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}
