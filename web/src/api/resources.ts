import api from './client';
import type {
  Group, GroupMember, Membership, User, Provider, ProviderHealth,
  RoutingRule, DashboardStats, TimeSeriesPoint,
  UsageByUser, UsageByProvider, PaginatedMessages,
  MessageDetail, ActivityLog,
} from '../types/api';

// Groups
export const fetchGroups = async (): Promise<Group[]> => (await api.get('/groups')).data;
export const fetchGroup = async (id: string): Promise<Group> => (await api.get(`/groups/${id}`)).data;
export const createGroup = async (name: string, monthly_limit?: number): Promise<Group> =>
  (await api.post('/groups', { name, monthly_limit })).data;
export const updateGroup = async (id: string, data: { name?: string; monthly_limit?: number }): Promise<Group> =>
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
  data: { username: string; email?: string; allowed_domains?: string[] }
): Promise<User> => (await api.post(`/groups/${groupId}/service-accounts`, data)).data;

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
export const resetUserPassword = async (id: string, new_password: string): Promise<void> =>
  { await api.post(`/users/${id}/reset-password`, { new_password }); };
export const fetchUserMemberships = async (id: string): Promise<Membership[]> =>
  (await api.get(`/users/${id}/memberships`)).data;
export const updatePasswordDisabled = async (id: string, password_disabled: boolean): Promise<User> =>
  (await api.patch(`/users/${id}/password-disabled`, { password_disabled })).data;

// Providers
export const fetchProviders = async (): Promise<Provider[]> => (await api.get('/providers')).data;
export const fetchProvider = async (id: string): Promise<Provider> => (await api.get(`/providers/${id}`)).data;
export const createProvider = async (data: Record<string, unknown>): Promise<Provider> =>
  (await api.post('/providers', data)).data;
export const updateProvider = async (id: string, data: Record<string, unknown>): Promise<Provider> =>
  (await api.put(`/providers/${id}`, data)).data;
export const deleteProvider = async (id: string): Promise<void> => { await api.delete(`/providers/${id}`); };
export const fetchProviderHealth = async (id: string): Promise<ProviderHealth> =>
  (await api.get(`/providers/${id}/health`)).data;

// Routing Rules
export const fetchRoutingRules = async (): Promise<RoutingRule[]> => (await api.get('/routing-rules')).data;
export const fetchRoutingRule = async (id: string): Promise<RoutingRule> => (await api.get(`/routing-rules/${id}`)).data;
export const createRoutingRule = async (data: Record<string, unknown>): Promise<RoutingRule> =>
  (await api.post('/routing-rules', data)).data;
export const updateRoutingRule = async (id: string, data: Record<string, unknown>): Promise<RoutingRule> =>
  (await api.put(`/routing-rules/${id}`, data)).data;
export const deleteRoutingRule = async (id: string): Promise<void> => { await api.delete(`/routing-rules/${id}`); };

// Stats
export const fetchDashboardStats = async (from?: string, to?: string): Promise<DashboardStats> =>
  (await api.get('/stats/dashboard', { params: { from, to } })).data;
export const fetchTimeSeries = async (from?: string, to?: string): Promise<TimeSeriesPoint[]> =>
  (await api.get('/stats/timeseries', { params: { from, to } })).data;
export const fetchUsageByUser = async (from?: string, to?: string): Promise<UsageByUser[]> =>
  (await api.get('/stats/by-user', { params: { from, to } })).data;
export const fetchUsageByProvider = async (from?: string, to?: string): Promise<UsageByProvider[]> =>
  (await api.get('/stats/by-provider', { params: { from, to } })).data;

// Messages
export const fetchMessages = async (page = 1, pageSize = 20, status?: string): Promise<PaginatedMessages> =>
  (await api.get('/messages', { params: { page, page_size: pageSize, status } })).data;
export const fetchMessage = async (id: string): Promise<MessageDetail> =>
  (await api.get(`/messages/${id}`)).data;
