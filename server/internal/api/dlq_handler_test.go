package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDLQReprocessHandler_Unauthorized(t *testing.T) {
	// No auth context set -- AccountFromContext returns uuid.Nil
	body := `{"message_ids":["id1","id2"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dlq/reprocess", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := DLQReprocessHandler(nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got %q", resp["error"])
	}
}

func TestDLQReprocessHandler_InvalidJSON(t *testing.T) {
	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dlq/reprocess", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	ctx := setAuthContext(req.Context(), accountID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler := DLQReprocessHandler(nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid request body" {
		t.Errorf("expected error 'invalid request body', got %q", resp["error"])
	}
}

func TestDLQReprocessHandler_EmptyMessageIDs(t *testing.T) {
	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	body := `{"message_ids":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dlq/reprocess", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setAuthContext(req.Context(), accountID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler := DLQReprocessHandler(nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "message_ids is required and must not be empty" {
		t.Errorf("expected error about empty message_ids, got %q", resp["error"])
	}
}

func TestDLQReprocessHandler_MissingMessageIDsField(t *testing.T) {
	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// Valid JSON but no message_ids field -- Go zero-value for []string is nil, len 0
	body := `{"reset_retry_count":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dlq/reprocess", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := setAuthContext(req.Context(), accountID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler := DLQReprocessHandler(nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}
