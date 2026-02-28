package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

func TestCreateGroupHandler_Valid(t *testing.T) {
	grp := testGroup()
	mock := &mockQuerier{
		createGroupFn: func(ctx context.Context, arg storage.CreateGroupParams) (storage.Group, error) {
			if arg.Name != "new-company" {
				t.Errorf("expected name new-company, got %s", arg.Name)
			}
			if arg.GroupType != "company" {
				t.Errorf("expected group_type company, got %s", arg.GroupType)
			}
			return grp, nil
		},
	}

	body := `{"name":"new-company"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler := CreateGroupHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp groupResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Name != grp.Name {
		t.Errorf("expected name %s, got %s", grp.Name, resp.Name)
	}
}

func TestCreateGroupHandler_WithMonthlyLimit(t *testing.T) {
	grp := testGroup()
	grp.MonthlyLimit = 50000
	updateCalled := false

	mock := &mockQuerier{
		createGroupFn: func(ctx context.Context, arg storage.CreateGroupParams) (storage.Group, error) {
			return grp, nil
		},
		updateGroupFn: func(ctx context.Context, arg storage.UpdateGroupParams) (storage.Group, error) {
			updateCalled = true
			if arg.MonthlyLimit != 50000 {
				t.Errorf("expected monthly_limit 50000, got %d", arg.MonthlyLimit)
			}
			grp.MonthlyLimit = arg.MonthlyLimit
			return grp, nil
		},
	}

	body := `{"name":"new-company","monthly_limit":50000}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler := CreateGroupHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if !updateCalled {
		t.Error("expected UpdateGroup to be called for monthly_limit")
	}
}

func TestCreateGroupHandler_MissingName(t *testing.T) {
	mock := &mockQuerier{}

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler := CreateGroupHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestListGroupsHandler(t *testing.T) {
	grp := testGroup()
	mock := &mockQuerier{
		listGroupsFn: func(ctx context.Context) ([]storage.Group, error) {
			return []storage.Group{grp}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
	rec := httptest.NewRecorder()

	handler := ListGroupsHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []groupResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 group, got %d", len(resp))
	}
}

func TestGetGroupHandler_Found(t *testing.T) {
	grp := testGroup()
	mock := &mockQuerier{
		getGroupByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Group, error) {
			return grp, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	// Set JWT context with matching group
	ctx = setJWTContext(ctx, testUser().ID, grp.ID, "admin", "organization")
	req = req.WithContext(ctx)

	handler := GetGroupHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetGroupHandler_SystemAdmin(t *testing.T) {
	grp := testGroup()
	mock := &mockQuerier{
		getGroupByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Group, error) {
			return grp, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	// System admin can access any group
	systemGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	ctx = setJWTContext(ctx, testUser().ID, systemGroupID, "admin", "system")
	req = req.WithContext(ctx)

	handler := GetGroupHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetGroupHandler_Forbidden(t *testing.T) {
	grp := testGroup()
	mock := &mockQuerier{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	// Different group, not system
	otherGroupID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	ctx = setJWTContext(ctx, testUser().ID, otherGroupID, "admin", "company")
	req = req.WithContext(ctx)

	handler := GetGroupHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestDeleteGroupHandler_Success(t *testing.T) {
	grp := testGroup()
	grp.GroupType = "company"
	mock := &mockQuerier{
		getGroupByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Group, error) {
			return grp, nil
		},
		updateGroupStatusFn: func(ctx context.Context, arg storage.UpdateGroupStatusParams) (storage.Group, error) {
			if arg.Status != "deleted" {
				t.Errorf("expected status deleted, got %s", arg.Status)
			}
			grp.Status = arg.Status
			return grp, nil
		},
		listGroupMembersByGroupIDFn: func(ctx context.Context, groupID uuid.UUID) ([]storage.GroupMember, error) {
			return nil, nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+grp.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := DeleteGroupHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteGroupHandler_SystemGroupForbidden(t *testing.T) {
	grp := testGroup()
	grp.GroupType = "system"
	mock := &mockQuerier{
		getGroupByIDFn: func(ctx context.Context, id uuid.UUID) (storage.Group, error) {
			return grp, nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+grp.ID.String(), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler := DeleteGroupHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestListGroupMembersHandler(t *testing.T) {
	grp := testGroup()
	member := testGroupMember()
	mock := &mockQuerier{
		listGroupMembersByGroupIDFn: func(ctx context.Context, groupID uuid.UUID) ([]storage.GroupMember, error) {
			return []storage.GroupMember{member}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+grp.ID.String()+"/members", nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = setJWTContext(ctx, testUser().ID, grp.ID, "admin", "organization")
	req = req.WithContext(ctx)

	handler := ListGroupMembersHandler(mock)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []groupMemberResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 member, got %d", len(resp))
	}
}

func TestAddGroupMemberHandler_Valid(t *testing.T) {
	grp := testGroup()
	member := testGroupMember()
	usr := testUser()
	usr.AccountType = "user"

	mock := &mockQuerier{
		getUserByIDFn: func(ctx context.Context, id uuid.UUID) (storage.User, error) {
			return usr, nil
		},
		listGroupsByUserIDFn: func(ctx context.Context, userID uuid.UUID) ([]storage.Group, error) {
			return nil, nil
		},
		createGroupMemberFn: func(ctx context.Context, arg storage.CreateGroupMemberParams) (storage.GroupMember, error) {
			if arg.GroupID != grp.ID {
				t.Errorf("expected group ID %s, got %s", grp.ID, arg.GroupID)
			}
			return member, nil
		},
	}

	body := `{"user_id":"` + usr.ID.String() + `","role":"member"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups/"+grp.ID.String()+"/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler := AddGroupMemberHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAddGroupMemberHandler_SMTPAlreadyInGroup(t *testing.T) {
	grp := testGroup()
	usr := testUser()
	usr.AccountType = "smtp"

	mock := &mockQuerier{
		getUserByIDFn: func(ctx context.Context, id uuid.UUID) (storage.User, error) {
			return usr, nil
		},
		listGroupsByUserIDFn: func(ctx context.Context, userID uuid.UUID) ([]storage.Group, error) {
			return []storage.Group{grp}, nil
		},
	}

	body := `{"user_id":"` + usr.ID.String() + `","role":"member"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups/"+grp.ID.String()+"/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler := AddGroupMemberHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rec.Code)
	}
}

func TestUpdateGroupMemberRoleHandler_Valid(t *testing.T) {
	grp := testGroup()
	member := testGroupMember()
	member.Role = "member"

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			return member, nil
		},
		updateGroupMemberRoleFn: func(ctx context.Context, arg storage.UpdateGroupMemberRoleParams) (storage.GroupMember, error) {
			member.Role = arg.Role
			return member, nil
		},
	}

	body := `{"role":"admin"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/groups/"+grp.ID.String()+"/members/"+member.UserID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	rctx.URLParams.Add("uid", member.UserID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler := UpdateGroupMemberRoleHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateGroupMemberRoleHandler_LastOwner(t *testing.T) {
	grp := testGroup()
	member := testGroupMember()
	member.Role = "owner"

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			return member, nil
		},
		countGroupOwnersFn: func(ctx context.Context, groupID uuid.UUID) (int64, error) {
			return 1, nil
		},
	}

	body := `{"role":"member"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/groups/"+grp.ID.String()+"/members/"+member.UserID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	rctx.URLParams.Add("uid", member.UserID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler := UpdateGroupMemberRoleHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rec.Code)
	}
}

func TestRemoveGroupMemberHandler_Success(t *testing.T) {
	grp := testGroup()
	member := testGroupMember()
	member.Role = "member"
	deleteCalled := false

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			return member, nil
		},
		deleteGroupMemberFn: func(ctx context.Context, id uuid.UUID) error {
			deleteCalled = true
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+grp.ID.String()+"/members/"+member.UserID.String(), nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	rctx.URLParams.Add("uid", member.UserID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler := RemoveGroupMemberHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if !deleteCalled {
		t.Error("expected DeleteGroupMember to be called")
	}
}

func TestRemoveGroupMemberHandler_LastOwner(t *testing.T) {
	grp := testGroup()
	member := testGroupMember()
	member.Role = "owner"

	mock := &mockQuerier{
		getGroupMemberByUserAndGroupFn: func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
			return member, nil
		},
		countGroupOwnersFn: func(ctx context.Context, groupID uuid.UUID) (int64, error) {
			return 1, nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+grp.ID.String()+"/members/"+member.UserID.String(), nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grp.ID.String())
	rctx.URLParams.Add("uid", member.UserID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler := RemoveGroupMemberHandler(mock, nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rec.Code)
	}
}
