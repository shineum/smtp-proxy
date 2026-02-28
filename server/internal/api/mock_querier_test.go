package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// errNotFound is a sentinel error for not-found simulation.
var errNotFound = errors.New("not found")

// mockQuerier implements storage.Querier for testing.
type mockQuerier struct {
	// User methods
	createUserFn       func(ctx context.Context, arg storage.CreateUserParams) (storage.User, error)
	getUserByIDFn      func(ctx context.Context, id uuid.UUID) (storage.User, error)
	getUserByEmailFn   func(ctx context.Context, email string) (storage.User, error)
	getUserByUsernameFn func(ctx context.Context, username sql.NullString) (storage.User, error)
	getUserByAPIKeyFn  func(ctx context.Context, apiKey sql.NullString) (storage.User, error)
	listUsersFn        func(ctx context.Context) ([]storage.User, error)
	updateUserFn       func(ctx context.Context, arg storage.UpdateUserParams) (storage.User, error)
	updateUserStatusFn func(ctx context.Context, arg storage.UpdateUserStatusParams) (storage.User, error)
	deleteUserFn       func(ctx context.Context, id uuid.UUID) error

	// Group methods
	createGroupFn       func(ctx context.Context, arg storage.CreateGroupParams) (storage.Group, error)
	getGroupByIDFn      func(ctx context.Context, id uuid.UUID) (storage.Group, error)
	getGroupByNameFn    func(ctx context.Context, name string) (storage.Group, error)
	listGroupsFn        func(ctx context.Context) ([]storage.Group, error)
	updateGroupFn       func(ctx context.Context, arg storage.UpdateGroupParams) (storage.Group, error)
	updateGroupStatusFn func(ctx context.Context, arg storage.UpdateGroupStatusParams) (storage.Group, error)
	deleteGroupFn       func(ctx context.Context, id uuid.UUID) error

	// GroupMember methods
	createGroupMemberFn            func(ctx context.Context, arg storage.CreateGroupMemberParams) (storage.GroupMember, error)
	getGroupMemberByIDFn           func(ctx context.Context, id uuid.UUID) (storage.GroupMember, error)
	getGroupMemberByUserAndGroupFn func(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error)
	listGroupMembersByGroupIDFn    func(ctx context.Context, groupID uuid.UUID) ([]storage.GroupMember, error)
	listGroupsByUserIDFn           func(ctx context.Context, userID uuid.UUID) ([]storage.Group, error)
	updateGroupMemberRoleFn        func(ctx context.Context, arg storage.UpdateGroupMemberRoleParams) (storage.GroupMember, error)
	deleteGroupMemberFn            func(ctx context.Context, id uuid.UUID) error
	countGroupOwnersFn             func(ctx context.Context, groupID uuid.UUID) (int64, error)

	// Provider methods
	createProviderFn      func(ctx context.Context, arg storage.CreateProviderParams) (storage.EspProvider, error)
	getProviderByIDFn     func(ctx context.Context, id uuid.UUID) (storage.EspProvider, error)
	listProvidersByGroupFn func(ctx context.Context, groupID uuid.UUID) ([]storage.EspProvider, error)
	updateProviderFn      func(ctx context.Context, arg storage.UpdateProviderParams) (storage.EspProvider, error)
	deleteProviderFn      func(ctx context.Context, id uuid.UUID) error

	// Routing Rule methods
	createRoutingRuleFn      func(ctx context.Context, arg storage.CreateRoutingRuleParams) (storage.RoutingRule, error)
	getRoutingRuleByIDFn     func(ctx context.Context, id uuid.UUID) (storage.RoutingRule, error)
	listRoutingRulesByGroupFn func(ctx context.Context, groupID uuid.UUID) ([]storage.RoutingRule, error)
	updateRoutingRuleFn      func(ctx context.Context, arg storage.UpdateRoutingRuleParams) (storage.RoutingRule, error)
	deleteRoutingRuleFn      func(ctx context.Context, id uuid.UUID) error

	// ActivityLog methods
	createActivityLogFn          func(ctx context.Context, arg storage.CreateActivityLogParams) (storage.ActivityLog, error)
	listActivityLogsByGroupIDFn  func(ctx context.Context, arg storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error)

	// DeliveryLog methods
	getDeliveryLogByProviderMessageIDFn func(ctx context.Context, providerMessageID sql.NullString) (storage.DeliveryLog, error)
	updateDeliveryLogStatusFn           func(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error

	// Session methods
	createSessionFn      func(ctx context.Context, arg storage.CreateSessionParams) (storage.Session, error)
	getSessionByIDFn     func(ctx context.Context, id uuid.UUID) (storage.Session, error)
	deleteSessionFn      func(ctx context.Context, id uuid.UUID) error
	listSessionsByUserIDFn func(ctx context.Context, userID uuid.UUID) ([]storage.Session, error)
}

// --- User methods ---

func (m *mockQuerier) CreateUser(ctx context.Context, arg storage.CreateUserParams) (storage.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, arg)
	}
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (storage.User, error) {
	if m.getUserByIDFn != nil {
		return m.getUserByIDFn(ctx, id)
	}
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByEmail(ctx context.Context, email string) (storage.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByUsername(ctx context.Context, username sql.NullString) (storage.User, error) {
	if m.getUserByUsernameFn != nil {
		return m.getUserByUsernameFn(ctx, username)
	}
	return storage.User{}, nil
}

func (m *mockQuerier) GetUserByAPIKey(ctx context.Context, apiKey sql.NullString) (storage.User, error) {
	if m.getUserByAPIKeyFn != nil {
		return m.getUserByAPIKeyFn(ctx, apiKey)
	}
	return storage.User{}, nil
}

func (m *mockQuerier) ListUsers(ctx context.Context) ([]storage.User, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateUser(ctx context.Context, arg storage.UpdateUserParams) (storage.User, error) {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, arg)
	}
	return storage.User{}, nil
}

func (m *mockQuerier) UpdateUserPassword(_ context.Context, _ storage.UpdateUserPasswordParams) error {
	return nil
}

func (m *mockQuerier) UpdateUserStatus(ctx context.Context, arg storage.UpdateUserStatusParams) (storage.User, error) {
	if m.updateUserStatusFn != nil {
		return m.updateUserStatusFn(ctx, arg)
	}
	return storage.User{}, nil
}

func (m *mockQuerier) UpdateUserLastLogin(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, id)
	}
	return nil
}

func (m *mockQuerier) IncrementFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) ResetFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

// --- Group methods ---

func (m *mockQuerier) CreateGroup(ctx context.Context, arg storage.CreateGroupParams) (storage.Group, error) {
	if m.createGroupFn != nil {
		return m.createGroupFn(ctx, arg)
	}
	return storage.Group{}, nil
}

func (m *mockQuerier) GetGroupByID(ctx context.Context, id uuid.UUID) (storage.Group, error) {
	if m.getGroupByIDFn != nil {
		return m.getGroupByIDFn(ctx, id)
	}
	return storage.Group{}, nil
}

func (m *mockQuerier) GetGroupByName(ctx context.Context, name string) (storage.Group, error) {
	if m.getGroupByNameFn != nil {
		return m.getGroupByNameFn(ctx, name)
	}
	return storage.Group{}, nil
}

func (m *mockQuerier) ListGroups(ctx context.Context) ([]storage.Group, error) {
	if m.listGroupsFn != nil {
		return m.listGroupsFn(ctx)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateGroup(ctx context.Context, arg storage.UpdateGroupParams) (storage.Group, error) {
	if m.updateGroupFn != nil {
		return m.updateGroupFn(ctx, arg)
	}
	return storage.Group{}, nil
}

func (m *mockQuerier) UpdateGroupStatus(ctx context.Context, arg storage.UpdateGroupStatusParams) (storage.Group, error) {
	if m.updateGroupStatusFn != nil {
		return m.updateGroupStatusFn(ctx, arg)
	}
	return storage.Group{}, nil
}

func (m *mockQuerier) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	if m.deleteGroupFn != nil {
		return m.deleteGroupFn(ctx, id)
	}
	return nil
}

func (m *mockQuerier) IncrementMonthlySent(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) ResetMonthlySent(_ context.Context, _ uuid.UUID) error {
	return nil
}

// --- GroupMember methods ---

func (m *mockQuerier) CreateGroupMember(ctx context.Context, arg storage.CreateGroupMemberParams) (storage.GroupMember, error) {
	if m.createGroupMemberFn != nil {
		return m.createGroupMemberFn(ctx, arg)
	}
	return storage.GroupMember{}, nil
}

func (m *mockQuerier) GetGroupMemberByID(ctx context.Context, id uuid.UUID) (storage.GroupMember, error) {
	if m.getGroupMemberByIDFn != nil {
		return m.getGroupMemberByIDFn(ctx, id)
	}
	return storage.GroupMember{}, nil
}

func (m *mockQuerier) GetGroupMemberByUserAndGroup(ctx context.Context, arg storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
	if m.getGroupMemberByUserAndGroupFn != nil {
		return m.getGroupMemberByUserAndGroupFn(ctx, arg)
	}
	return storage.GroupMember{}, nil
}

func (m *mockQuerier) ListGroupMembersByGroupID(ctx context.Context, groupID uuid.UUID) ([]storage.GroupMember, error) {
	if m.listGroupMembersByGroupIDFn != nil {
		return m.listGroupMembersByGroupIDFn(ctx, groupID)
	}
	return nil, nil
}

func (m *mockQuerier) ListGroupsByUserID(ctx context.Context, userID uuid.UUID) ([]storage.Group, error) {
	if m.listGroupsByUserIDFn != nil {
		return m.listGroupsByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateGroupMemberRole(ctx context.Context, arg storage.UpdateGroupMemberRoleParams) (storage.GroupMember, error) {
	if m.updateGroupMemberRoleFn != nil {
		return m.updateGroupMemberRoleFn(ctx, arg)
	}
	return storage.GroupMember{}, nil
}

func (m *mockQuerier) DeleteGroupMember(ctx context.Context, id uuid.UUID) error {
	if m.deleteGroupMemberFn != nil {
		return m.deleteGroupMemberFn(ctx, id)
	}
	return nil
}

func (m *mockQuerier) DeleteGroupMembersByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) CountGroupOwners(ctx context.Context, groupID uuid.UUID) (int64, error) {
	if m.countGroupOwnersFn != nil {
		return m.countGroupOwnersFn(ctx, groupID)
	}
	return 0, nil
}

// --- Provider methods ---

func (m *mockQuerier) CreateProvider(ctx context.Context, arg storage.CreateProviderParams) (storage.EspProvider, error) {
	if m.createProviderFn != nil {
		return m.createProviderFn(ctx, arg)
	}
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) GetProviderByID(ctx context.Context, id uuid.UUID) (storage.EspProvider, error) {
	if m.getProviderByIDFn != nil {
		return m.getProviderByIDFn(ctx, id)
	}
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) ListProvidersByGroupID(ctx context.Context, groupID uuid.UUID) ([]storage.EspProvider, error) {
	if m.listProvidersByGroupFn != nil {
		return m.listProvidersByGroupFn(ctx, groupID)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateProvider(ctx context.Context, arg storage.UpdateProviderParams) (storage.EspProvider, error) {
	if m.updateProviderFn != nil {
		return m.updateProviderFn(ctx, arg)
	}
	return storage.EspProvider{}, nil
}

func (m *mockQuerier) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	if m.deleteProviderFn != nil {
		return m.deleteProviderFn(ctx, id)
	}
	return nil
}

// --- Routing Rule methods ---

func (m *mockQuerier) CreateRoutingRule(ctx context.Context, arg storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
	if m.createRoutingRuleFn != nil {
		return m.createRoutingRuleFn(ctx, arg)
	}
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) GetRoutingRuleByID(ctx context.Context, id uuid.UUID) (storage.RoutingRule, error) {
	if m.getRoutingRuleByIDFn != nil {
		return m.getRoutingRuleByIDFn(ctx, id)
	}
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) ListRoutingRulesByGroupID(ctx context.Context, groupID uuid.UUID) ([]storage.RoutingRule, error) {
	if m.listRoutingRulesByGroupFn != nil {
		return m.listRoutingRulesByGroupFn(ctx, groupID)
	}
	return nil, nil
}

func (m *mockQuerier) UpdateRoutingRule(ctx context.Context, arg storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	if m.updateRoutingRuleFn != nil {
		return m.updateRoutingRuleFn(ctx, arg)
	}
	return storage.RoutingRule{}, nil
}

func (m *mockQuerier) DeleteRoutingRule(ctx context.Context, id uuid.UUID) error {
	if m.deleteRoutingRuleFn != nil {
		return m.deleteRoutingRuleFn(ctx, id)
	}
	return nil
}

// --- Message methods ---

func (m *mockQuerier) EnqueueMessage(_ context.Context, _ storage.EnqueueMessageParams) (storage.Message, error) {
	return storage.Message{}, nil
}

func (m *mockQuerier) EnqueueMessageMetadata(_ context.Context, _ storage.EnqueueMessageMetadataParams) (storage.Message, error) {
	return storage.Message{}, nil
}

func (m *mockQuerier) GetMessageByID(_ context.Context, _ uuid.UUID) (storage.Message, error) {
	return storage.Message{}, nil
}

func (m *mockQuerier) GetQueuedMessages(_ context.Context, _ int32) ([]storage.Message, error) {
	return nil, nil
}

func (m *mockQuerier) ListMessagesByGroupID(_ context.Context, _ storage.ListMessagesByGroupIDParams) ([]storage.Message, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateMessageStatus(_ context.Context, _ storage.UpdateMessageStatusParams) error {
	return nil
}

// --- DeliveryLog methods ---

func (m *mockQuerier) CreateDeliveryLog(_ context.Context, _ storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) GetDeliveryLogByMessageID(_ context.Context, _ uuid.UUID) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) GetDeliveryLogByProviderMessageID(ctx context.Context, providerMessageID sql.NullString) (storage.DeliveryLog, error) {
	if m.getDeliveryLogByProviderMessageIDFn != nil {
		return m.getDeliveryLogByProviderMessageIDFn(ctx, providerMessageID)
	}
	return storage.DeliveryLog{}, nil
}

func (m *mockQuerier) IncrementRetryCount(_ context.Context, _ storage.IncrementRetryCountParams) error {
	return nil
}

func (m *mockQuerier) ListDeliveryLogsByMessageID(_ context.Context, _ uuid.UUID) ([]storage.DeliveryLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListDeliveryLogsByGroupAndStatus(_ context.Context, _ storage.ListDeliveryLogsByGroupAndStatusParams) ([]storage.DeliveryLog, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateDeliveryLogStatus(ctx context.Context, arg storage.UpdateDeliveryLogStatusParams) error {
	if m.updateDeliveryLogStatusFn != nil {
		return m.updateDeliveryLogStatusFn(ctx, arg)
	}
	return nil
}

// --- Aggregate query methods ---

func (m *mockQuerier) AverageDeliveryDuration(_ context.Context, _ storage.AverageDeliveryDurationParams) ([]storage.AverageDeliveryDurationRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByGroup(_ context.Context, _ storage.CountDeliveryLogsByGroupParams) ([]storage.CountDeliveryLogsByGroupRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByProvider(_ context.Context, _ storage.CountDeliveryLogsByProviderParams) ([]storage.CountDeliveryLogsByProviderRow, error) {
	return nil, nil
}

func (m *mockQuerier) CountDeliveryLogsByStatus(_ context.Context, _ storage.CountDeliveryLogsByStatusParams) ([]storage.CountDeliveryLogsByStatusRow, error) {
	return nil, nil
}

// --- ActivityLog methods ---

func (m *mockQuerier) CreateActivityLog(ctx context.Context, arg storage.CreateActivityLogParams) (storage.ActivityLog, error) {
	if m.createActivityLogFn != nil {
		return m.createActivityLogFn(ctx, arg)
	}
	return storage.ActivityLog{}, nil
}

func (m *mockQuerier) GetActivityLogByID(_ context.Context, _ uuid.UUID) (storage.ActivityLog, error) {
	return storage.ActivityLog{}, nil
}

func (m *mockQuerier) ListActivityLogsByGroupID(ctx context.Context, arg storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error) {
	if m.listActivityLogsByGroupIDFn != nil {
		return m.listActivityLogsByGroupIDFn(ctx, arg)
	}
	return nil, nil
}

func (m *mockQuerier) ListActivityLogsByActorID(_ context.Context, _ storage.ListActivityLogsByActorIDParams) ([]storage.ActivityLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListActivityLogsByResource(_ context.Context, _ storage.ListActivityLogsByResourceParams) ([]storage.ActivityLog, error) {
	return nil, nil
}

// --- Session methods ---

func (m *mockQuerier) CreateSession(ctx context.Context, arg storage.CreateSessionParams) (storage.Session, error) {
	if m.createSessionFn != nil {
		return m.createSessionFn(ctx, arg)
	}
	return storage.Session{}, nil
}

func (m *mockQuerier) GetSessionByID(ctx context.Context, id uuid.UUID) (storage.Session, error) {
	if m.getSessionByIDFn != nil {
		return m.getSessionByIDFn(ctx, id)
	}
	return storage.Session{}, nil
}

func (m *mockQuerier) DeleteSession(ctx context.Context, id uuid.UUID) error {
	if m.deleteSessionFn != nil {
		return m.deleteSessionFn(ctx, id)
	}
	return nil
}

func (m *mockQuerier) DeleteExpiredSessions(_ context.Context) error {
	return nil
}

func (m *mockQuerier) DeleteSessionsByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockQuerier) ListSessionsByUserID(ctx context.Context, userID uuid.UUID) ([]storage.Session, error) {
	if m.listSessionsByUserIDFn != nil {
		return m.listSessionsByUserIDFn(ctx, userID)
	}
	return nil, nil
}

// --- Context helpers ---

// setAuthContext injects the account ID into context the same way the BearerAuth
// middleware does, so that auth.AccountFromContext can retrieve it.
func setAuthContext(ctx context.Context, accountID uuid.UUID) context.Context {
	lookup := func(_ context.Context, _ string) (uuid.UUID, error) {
		return accountID, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer fakekey")
	req = req.WithContext(ctx)

	var resultCtx context.Context
	captured := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resultCtx = r.Context()
	})
	auth.BearerAuth(lookup)(captured).ServeHTTP(httptest.NewRecorder(), req)

	return resultCtx
}

// setJWTContext injects JWT-style user/group context values the same way the
// JWTAuth middleware does, so that auth.GroupIDFromContext, auth.UserFromContext,
// auth.RoleFromContext, and auth.GroupTypeFromContext can retrieve them.
func setJWTContext(ctx context.Context, userID, groupID uuid.UUID, role, groupType string) context.Context {
	// We pass a fake request through the real JWTAuth middleware.
	// To avoid needing a real JWT, we create a JWTService with a known secret,
	// generate a real token, and let the middleware parse it.
	jwtSvc := auth.NewJWTService(auth.JWTConfig{
		SigningKey:         "test-secret-key-that-is-long-enough-32",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	})

	token, err := jwtSvc.GenerateAccessToken(userID, groupID, "test@example.com", role, groupType)
	if err != nil {
		panic("setJWTContext: failed to generate token: " + err.Error())
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = req.WithContext(ctx)

	var resultCtx context.Context
	captured := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resultCtx = r.Context()
	})
	auth.JWTAuth(jwtSvc)(captured).ServeHTTP(httptest.NewRecorder(), req)

	if resultCtx == nil {
		panic("setJWTContext: middleware did not call next handler")
	}

	return resultCtx
}

// --- Test helpers ---

// testGroup returns a sample Group for testing.
func testGroup() storage.Group {
	return storage.Group{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		Name:         "test-group",
		Status:       "active",
		MonthlyLimit: 10000,
		MonthlySent:  0,
		CreatedAt:    pgtype.Timestamptz{Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Valid: true},
		GroupType:    "organization",
	}
}

// testGroupMember returns a sample GroupMember for testing.
func testGroupMember() storage.GroupMember {
	return storage.GroupMember{
		ID:        uuid.MustParse("00000000-0000-0000-0000-000000000011"),
		GroupID:   uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		UserID:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Role:      "owner",
		CreatedAt: pgtype.Timestamptz{Valid: true},
	}
}

// testUser returns a sample User for testing.
func testUser() storage.User {
	return storage.User{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Email:        "test@example.com",
		PasswordHash: "$2a$12$fakehash",
		Status:       "active",
		CreatedAt:    pgtype.Timestamptz{Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Valid: true},
		AccountType:  "user",
		ApiKey:       sql.NullString{String: "testapikey123", Valid: true},
	}
}

// testProvider returns a sample EspProvider for testing.
func testProvider() storage.EspProvider {
	return storage.EspProvider{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		GroupID:      uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		Name:         "test-provider",
		ProviderType: storage.ProviderTypeSendgrid,
		ApiKey:       sql.NullString{String: "sg-key-123", Valid: true},
		SmtpConfig:   []byte(`{}`),
		Enabled:      true,
		CreatedAt:    pgtype.Timestamptz{Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Valid: true},
	}
}

// testRoutingRule returns a sample RoutingRule for testing.
func testRoutingRule() storage.RoutingRule {
	return storage.RoutingRule{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		GroupID:    uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		Priority:   10,
		Conditions: []byte(`{"from":"*@example.com"}`),
		ProviderID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Enabled:    true,
		CreatedAt:  pgtype.Timestamptz{Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Valid: true},
	}
}

// Compile-time verification that mockQuerier implements storage.Querier.
var _ storage.Querier = (*mockQuerier)(nil)
