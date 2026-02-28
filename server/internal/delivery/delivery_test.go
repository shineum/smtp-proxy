package delivery

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// mockQuerier implements storage.Querier for testing.
type mockQuerier struct {
	updateStatusFn    func(ctx context.Context, arg storage.UpdateMessageStatusParams) error
	createDeliveryFn  func(ctx context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error)
	listProvidersFn   func(ctx context.Context, groupID uuid.UUID) ([]storage.EspProvider, error)
	capturedStatus    storage.MessageStatus
	capturedLogParams storage.CreateDeliveryLogParams
}

// ActivityLog methods.
func (m *mockQuerier) CreateActivityLog(_ context.Context, _ storage.CreateActivityLogParams) (storage.ActivityLog, error) {
	return storage.ActivityLog{}, nil
}
func (m *mockQuerier) GetActivityLogByID(_ context.Context, _ uuid.UUID) (storage.ActivityLog, error) {
	return storage.ActivityLog{}, nil
}
func (m *mockQuerier) ListActivityLogsByActorID(_ context.Context, _ storage.ListActivityLogsByActorIDParams) ([]storage.ActivityLog, error) {
	return nil, nil
}
func (m *mockQuerier) ListActivityLogsByGroupID(_ context.Context, _ storage.ListActivityLogsByGroupIDParams) ([]storage.ActivityLog, error) {
	return nil, nil
}
func (m *mockQuerier) ListActivityLogsByResource(_ context.Context, _ storage.ListActivityLogsByResourceParams) ([]storage.ActivityLog, error) {
	return nil, nil
}

// DeliveryLog methods.
func (m *mockQuerier) CreateDeliveryLog(ctx context.Context, arg storage.CreateDeliveryLogParams) (storage.DeliveryLog, error) {
	m.capturedLogParams = arg
	if m.createDeliveryFn != nil {
		return m.createDeliveryFn(ctx, arg)
	}
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) GetDeliveryLogByMessageID(_ context.Context, _ uuid.UUID) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) GetDeliveryLogByProviderMessageID(_ context.Context, _ sql.NullString) (storage.DeliveryLog, error) {
	return storage.DeliveryLog{}, nil
}
func (m *mockQuerier) ListDeliveryLogsByMessageID(_ context.Context, _ uuid.UUID) ([]storage.DeliveryLog, error) {
	return nil, nil
}
func (m *mockQuerier) ListDeliveryLogsByGroupAndStatus(_ context.Context, _ storage.ListDeliveryLogsByGroupAndStatusParams) ([]storage.DeliveryLog, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateDeliveryLogStatus(_ context.Context, _ storage.UpdateDeliveryLogStatusParams) error {
	return nil
}
func (m *mockQuerier) CountDeliveryLogsByStatus(_ context.Context, _ storage.CountDeliveryLogsByStatusParams) ([]storage.CountDeliveryLogsByStatusRow, error) {
	return nil, nil
}
func (m *mockQuerier) CountDeliveryLogsByProvider(_ context.Context, _ storage.CountDeliveryLogsByProviderParams) ([]storage.CountDeliveryLogsByProviderRow, error) {
	return nil, nil
}
func (m *mockQuerier) CountDeliveryLogsByGroup(_ context.Context, _ storage.CountDeliveryLogsByGroupParams) ([]storage.CountDeliveryLogsByGroupRow, error) {
	return nil, nil
}
func (m *mockQuerier) AverageDeliveryDuration(_ context.Context, _ storage.AverageDeliveryDurationParams) ([]storage.AverageDeliveryDurationRow, error) {
	return nil, nil
}

// Group methods.
func (m *mockQuerier) CreateGroup(_ context.Context, _ storage.CreateGroupParams) (storage.Group, error) {
	return storage.Group{}, nil
}
func (m *mockQuerier) DeleteGroup(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetGroupByID(_ context.Context, _ uuid.UUID) (storage.Group, error) {
	return storage.Group{}, nil
}
func (m *mockQuerier) GetGroupByName(_ context.Context, _ string) (storage.Group, error) {
	return storage.Group{}, nil
}
func (m *mockQuerier) ListGroups(_ context.Context) ([]storage.Group, error) { return nil, nil }
func (m *mockQuerier) ListGroupsByUserID(_ context.Context, _ uuid.UUID) ([]storage.Group, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateGroup(_ context.Context, _ storage.UpdateGroupParams) (storage.Group, error) {
	return storage.Group{}, nil
}
func (m *mockQuerier) UpdateGroupStatus(_ context.Context, _ storage.UpdateGroupStatusParams) (storage.Group, error) {
	return storage.Group{}, nil
}
func (m *mockQuerier) CountGroupOwners(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// GroupMember methods.
func (m *mockQuerier) CreateGroupMember(_ context.Context, _ storage.CreateGroupMemberParams) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
}
func (m *mockQuerier) DeleteGroupMember(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) DeleteGroupMembersByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *mockQuerier) GetGroupMemberByID(_ context.Context, _ uuid.UUID) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
}
func (m *mockQuerier) GetGroupMemberByUserAndGroup(_ context.Context, _ storage.GetGroupMemberByUserAndGroupParams) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
}
func (m *mockQuerier) ListGroupMembersByGroupID(_ context.Context, _ uuid.UUID) ([]storage.GroupMember, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateGroupMemberRole(_ context.Context, _ storage.UpdateGroupMemberRoleParams) (storage.GroupMember, error) {
	return storage.GroupMember{}, nil
}

// Message methods.
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
func (m *mockQuerier) IncrementRetryCount(_ context.Context, _ storage.IncrementRetryCountParams) error {
	return nil
}
func (m *mockQuerier) ListMessagesByGroupID(_ context.Context, _ storage.ListMessagesByGroupIDParams) ([]storage.Message, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateMessageStatus(ctx context.Context, arg storage.UpdateMessageStatusParams) error {
	m.capturedStatus = arg.Status
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, arg)
	}
	return nil
}

// Provider methods.
func (m *mockQuerier) CreateProvider(_ context.Context, _ storage.CreateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}
func (m *mockQuerier) DeleteProvider(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetProviderByID(_ context.Context, _ uuid.UUID) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}
func (m *mockQuerier) ListProvidersByGroupID(ctx context.Context, groupID uuid.UUID) ([]storage.EspProvider, error) {
	if m.listProvidersFn != nil {
		return m.listProvidersFn(ctx, groupID)
	}
	return nil, nil
}
func (m *mockQuerier) UpdateProvider(_ context.Context, _ storage.UpdateProviderParams) (storage.EspProvider, error) {
	return storage.EspProvider{}, nil
}

// RoutingRule methods.
func (m *mockQuerier) CreateRoutingRule(_ context.Context, _ storage.CreateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}
func (m *mockQuerier) DeleteRoutingRule(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetRoutingRuleByID(_ context.Context, _ uuid.UUID) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}
func (m *mockQuerier) ListRoutingRulesByGroupID(_ context.Context, _ uuid.UUID) ([]storage.RoutingRule, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateRoutingRule(_ context.Context, _ storage.UpdateRoutingRuleParams) (storage.RoutingRule, error) {
	return storage.RoutingRule{}, nil
}

// Session methods.
func (m *mockQuerier) CreateSession(_ context.Context, _ storage.CreateSessionParams) (storage.Session, error) {
	return storage.Session{}, nil
}
func (m *mockQuerier) DeleteSession(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) DeleteExpiredSessions(_ context.Context) error      { return nil }
func (m *mockQuerier) DeleteSessionsByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *mockQuerier) GetSessionByID(_ context.Context, _ uuid.UUID) (storage.Session, error) {
	return storage.Session{}, nil
}
func (m *mockQuerier) ListSessionsByUserID(_ context.Context, _ uuid.UUID) ([]storage.Session, error) {
	return nil, nil
}

// User methods.
func (m *mockQuerier) CreateUser(_ context.Context, _ storage.CreateUserParams) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) DeleteUser(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) GetUserByAPIKey(_ context.Context, _ sql.NullString) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) GetUserByEmail(_ context.Context, _ string) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) GetUserByID(_ context.Context, _ uuid.UUID) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) GetUserByUsername(_ context.Context, _ sql.NullString) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) IncrementFailedAttempts(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) IncrementMonthlySent(_ context.Context, _ uuid.UUID) error    { return nil }
func (m *mockQuerier) ListUsers(_ context.Context) ([]storage.User, error)          { return nil, nil }
func (m *mockQuerier) ResetFailedAttempts(_ context.Context, _ uuid.UUID) error     { return nil }
func (m *mockQuerier) ResetMonthlySent(_ context.Context, _ uuid.UUID) error        { return nil }
func (m *mockQuerier) UpdateUser(_ context.Context, _ storage.UpdateUserParams) (storage.User, error) {
	return storage.User{}, nil
}
func (m *mockQuerier) UpdateUserLastLogin(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockQuerier) UpdateUserPassword(_ context.Context, _ storage.UpdateUserPasswordParams) error {
	return nil
}
func (m *mockQuerier) UpdateUserStatus(_ context.Context, _ storage.UpdateUserStatusParams) (storage.User, error) {
	return storage.User{}, nil
}

// Ensure mockQuerier satisfies the Querier interface at compile time.
var _ storage.Querier = (*mockQuerier)(nil)
