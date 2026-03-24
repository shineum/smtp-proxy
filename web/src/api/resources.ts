import api from './client';
import type {
  Group, GroupMember, Membership, User, Provider, ProviderHealth, ProviderAccess, ProviderUsage,
  RoutingRule, DashboardStats, TimeSeriesPoint,
  UsageByUser, UsageByGroup, UsageByProvider, PaginatedMessages,
  MessageDetail, ActivityLog, ApiKeyInfo,
  ProviderFallback, DomainRateLimit,
} from '../types/api';

// Groups
export const fetchGroups = async (): Promise<Group[]> => (await api.get('/groups')).data;
export const fetchGroup = async (id: string): Promise<Group> => (await api.get(`/groups/${id}`)).data;
export const createGroup = async (name: string, monthly_limit?: number, display_name?: string, description?: string): Promise<Group> =>
  (await api.post('/groups', { name, monthly_limit, display_name, description })).data;
export const updateGroup = async (id: string, data: { name?: string; monthly_limit?: number; display_name?: string; description?: string }): Promise<Group> =>
  (await api.put(`/groups/${id}`, data)).data;
export const deleteGroup = async (id: string): Promise<void> => { await api.delete(`/groups/${id}`); };

// Group members
export const fetchGroupMembers = async (groupId: string): Promise<GroupMember[]> =>
  (await api.get(`/groups/${groupId}/members`)).data;
export const addGroupMember = async (groupId: string, user_id: string, role: string): Promise<GroupMember> =>
  (await api.post(`/groups/${groupId}/members`, { user_id, role })).data;
export const updateMemberRole = async (groupId: string, userId: string, role: string): Promise<GroupMember> =>
  (await api.patch(`/groups/${groupId}/members/${userId}`, { role })).data;
export const removeMember = async (groupId: string, userId: string): Promise<void> =>
  { await api.delete(`/groups/${groupId}/members/${userId}`); };

// Service accounts (group-scoped)
export const createServiceAccount = async (
  groupId: string,
  data: { username: string; email?: string; allowed_domains?: string[]; provider_id?: string }
): Promise<User> => (await api.post(`/groups/${groupId}/service-accounts`, data)).data;

// API Keys for service accounts
export const fetchApiKeys = async (groupId: string, userId: string): Promise<ApiKeyInfo[]> =>
  (await api.get(`/groups/${groupId}/service-accounts/${userId}/api-keys`)).data;

export const createApiKey = async (
  groupId: string, userId: string,
  data: { label: string; api_key_expires_in?: string }
): Promise<ApiKeyInfo> =>
  (await api.post(`/groups/${groupId}/service-accounts/${userId}/api-keys`, data)).data;

export const updateApiKeyStatus = async (
  groupId: string, userId: string, keyId: string, is_active: boolean
): Promise<ApiKeyInfo> =>
  (await api.patch(`/groups/${groupId}/service-accounts/${userId}/api-keys/${keyId}`, { is_active })).data;

export const deleteApiKey = async (
  groupId: string, userId: string, keyId: string
): Promise<void> => { await api.delete(`/groups/${groupId}/service-accounts/${userId}/api-keys/${keyId}`); };

export const updateServiceAccount = async (
  groupId: string, userId: string,
  data: { allowed_domains?: string[]; provider_id?: string }
): Promise<User> => (await api.patch(`/groups/${groupId}/service-accounts/${userId}`, data)).data;

// Activity logs
export const fetchActivityLogs = async (groupId: string, limit = 50, offset = 0): Promise<ActivityLog[]> =>
  (await api.get(`/groups/${groupId}/activity`, { params: { limit, offset } })).data;

// Users
export const fetchUsers = async (): Promise<User[]> => (await api.get('/users')).data;
export const fetchUser = async (id: string): Promise<User> => (await api.get(`/users/${id}`)).data;
export const createUser = async (data: Record<string, unknown>): Promise<User> =>
  (await api.post('/users', data)).data;
export const updateUserStatus = async (id: string, status: string): Promise<User> =>
  (await api.patch(`/users/${id}/status`, { status })).data;
export const deleteUser = async (id: string): Promise<void> => { await api.delete(`/users/${id}`); };
export const restoreUser = async (id: string): Promise<User> =>
  (await api.post(`/users/${id}/restore`)).data;
export const fetchDeletedUsers = async (): Promise<User[]> =>
  (await api.get('/users/deleted')).data;
export const resetApiKey = async (id: string, api_key_expires_in?: string): Promise<User> =>
  (await api.post(`/users/${id}/reset-api-key`, { api_key_expires_in })).data;
export const resetServiceAccountApiKey = async (groupId: string, userId: string, api_key_expires_in?: string): Promise<User> =>
  (await api.post(`/groups/${groupId}/service-accounts/${userId}/reset-api-key`, { api_key_expires_in })).data;
export const resetUserPassword = async (id: string, new_password: string): Promise<void> =>
  { await api.post(`/users/${id}/reset-password`, { new_password }); };
export const fetchUserMemberships = async (id: string): Promise<Membership[]> =>
  (await api.get(`/users/${id}/memberships`)).data;
export const updatePasswordDisabled = async (id: string, password_disabled: boolean): Promise<User> =>
  (await api.patch(`/users/${id}/password-disabled`, { password_disabled })).data;

// Providers
export const fetchProviders = async (groupId?: string): Promise<Provider[]> =>
  (await api.get('/providers', { params: groupId ? { group_id: groupId } : undefined })).data;
export const fetchProvider = async (id: string): Promise<Provider> => (await api.get(`/providers/${id}`)).data;
export const createProvider = async (data: Record<string, unknown>): Promise<Provider> =>
  (await api.post('/providers', data)).data;
export const updateProvider = async (id: string, data: Record<string, unknown>): Promise<Provider> =>
  (await api.put(`/providers/${id}`, data)).data;
export const deleteProvider = async (id: string): Promise<void> => { await api.delete(`/providers/${id}`); };
export const fetchProviderHealth = async (id: string): Promise<ProviderHealth> =>
  (await api.get(`/providers/${id}/health`)).data;
export const fetchProviderAccess = async (id: string): Promise<ProviderAccess[]> =>
  (await api.get(`/providers/${id}/access`)).data;
export const grantProviderAccess = async (id: string, group_id: string): Promise<void> =>
  { await api.post(`/providers/${id}/access`, { group_id }); };
export const revokeProviderAccess = async (id: string, groupId: string): Promise<void> =>
  { await api.delete(`/providers/${id}/access/${groupId}`); };
export const fetchProviderUsage = async (id: string): Promise<ProviderUsage[]> =>
  (await api.get(`/providers/${id}/usage`)).data;
export const sendTestEmail = async (
  id: string, data: { from: string; to: string; subject: string; body: string }
): Promise<{ success: boolean; provider_message_id?: string; error?: string; duration_ms: number }> =>
  (await api.post(`/providers/${id}/send`, data)).data;

// Routing Rules
export const fetchRoutingRules = async (): Promise<RoutingRule[]> => (await api.get('/routing-rules')).data;
export const fetchRoutingRule = async (id: string): Promise<RoutingRule> => (await api.get(`/routing-rules/${id}`)).data;
export const createRoutingRule = async (data: Record<string, unknown>): Promise<RoutingRule> =>
  (await api.post('/routing-rules', data)).data;
export const updateRoutingRule = async (id: string, data: Record<string, unknown>): Promise<RoutingRule> =>
  (await api.put(`/routing-rules/${id}`, data)).data;
export const deleteRoutingRule = async (id: string): Promise<void> => { await api.delete(`/routing-rules/${id}`); };

// Stats
export const fetchDashboardStats = async (from?: string, to?: string, group_id?: string): Promise<DashboardStats> =>
  (await api.get('/stats/dashboard', { params: { from, to, group_id } })).data;
export const fetchTimeSeries = async (from?: string, to?: string, group_id?: string): Promise<TimeSeriesPoint[]> =>
  (await api.get('/stats/timeseries', { params: { from, to, group_id } })).data;
export const fetchUsageByUser = async (from?: string, to?: string, group_id?: string): Promise<UsageByUser[]> =>
  (await api.get('/stats/by-user', { params: { from, to, group_id } })).data;
export const fetchUsageByGroup = async (from?: string, to?: string): Promise<UsageByGroup[]> =>
  (await api.get('/stats/by-group', { params: { from, to } })).data;
export const fetchUsageByProvider = async (from?: string, to?: string, group_id?: string): Promise<UsageByProvider[]> =>
  (await api.get('/stats/by-provider', { params: { from, to, group_id } })).data;

// Messages
export const fetchMessages = async (page = 1, pageSize = 20, status?: string): Promise<PaginatedMessages> =>
  (await api.get('/messages', { params: { page, page_size: pageSize, status } })).data;
export const fetchMessage = async (id: string): Promise<MessageDetail> =>
  (await api.get(`/messages/${id}`)).data;

// Provider Fallbacks
export const fetchProviderFallbacks = async (userId: string): Promise<ProviderFallback[]> =>
  (await api.get(`/users/${userId}/fallbacks`)).data;
export const createProviderFallback = async (
  userId: string, data: { provider_id: string; priority: number; enabled: boolean }
): Promise<ProviderFallback> =>
  (await api.post(`/users/${userId}/fallbacks`, data)).data;
export const updateProviderFallback = async (
  userId: string, fallbackId: string, data: { priority: number; enabled: boolean }
): Promise<ProviderFallback> =>
  (await api.put(`/users/${userId}/fallbacks/${fallbackId}`, data)).data;
export const deleteProviderFallback = async (userId: string, fallbackId: string): Promise<void> =>
  { await api.delete(`/users/${userId}/fallbacks/${fallbackId}`); };

// Domain Rate Limits
export const fetchDomainRateLimits = async (): Promise<DomainRateLimit[]> =>
  (await api.get('/domain-rate-limits')).data;
export const createDomainRateLimit = async (
  data: { domain: string; max_per_minute: number; max_per_hour: number; enabled: boolean }
): Promise<DomainRateLimit> =>
  (await api.post('/domain-rate-limits', data)).data;
export const updateDomainRateLimit = async (
  id: string, data: { max_per_minute: number; max_per_hour: number; enabled: boolean }
): Promise<DomainRateLimit> =>
  (await api.put(`/domain-rate-limits/${id}`, data)).data;
export const deleteDomainRateLimit = async (id: string): Promise<void> =>
  { await api.delete(`/domain-rate-limits/${id}`); };

// DLQ
export const reprocessDLQ = async (messageIds: string[]): Promise<{ reprocessed: number; total: number }> =>
  (await api.post('/dlq/reprocess', { message_ids: messageIds })).data;
