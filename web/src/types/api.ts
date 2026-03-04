export interface LoginRequest {
  email: string;
  password: string;
  group_id?: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
}

export interface User {
  id: string;
  email: string;
  username?: string;
  account_type: string;
  status: string;
  allowed_domains?: string[];
  api_key?: string;
  provider_id?: string;
  password_disabled: boolean;
  last_login?: string;
  created_at: string;
  updated_at: string;
}

export interface Group {
  id: string;
  name: string;
  group_type: string;
  status: string;
  monthly_limit: number;
  monthly_sent: number;
  created_at: string;
  updated_at: string;
}

export interface GroupMember {
  id: string;
  group_id: string;
  user_id: string;
  email?: string;
  username?: string;
  role: string;
  created_at: string;
}

export interface Membership {
  group_id: string;
  group_name: string;
  group_type: string;
  role: string;
  created_at: string;
}

export interface MeResponse {
  user: User;
  current_group: {
    group_id: string;
    group_type: string;
    role: string;
  };
  memberships: Membership[];
}

export interface Provider {
  id: string;
  group_id: string;
  name: string;
  provider_type: string;
  smtp_config: Record<string, unknown>;
  enabled: boolean;
  visibility: 'private' | 'shared' | 'global';
  created_at: string;
  updated_at: string;
}

export interface ProviderHealth {
  provider_id: string;
  status: string;
  enabled: boolean;
  sent_24h: number;
  failed_24h: number;
}

export interface RoutingRule {
  id: string;
  group_id: string;
  priority: number;
  conditions: Record<string, unknown>;
  provider_id: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface DashboardStats {
  total_messages: number;
  status_counts: Record<string, number>;
  success_rate: number;
  from: string;
  to: string;
}

export interface TimeSeriesPoint {
  day: string;
  status: string;
  count: number;
}

export interface UsageByUser {
  user_id: string;
  status: string;
  count: number;
}

export interface UsageByProvider {
  provider: string;
  status: string;
  count: number;
}

export interface Message {
  id: string;
  group_id?: string;
  user_id?: string;
  sender: string;
  recipients: string[];
  subject: string;
  status: string;
  enqueued_at?: string;
  processed_at?: string;
}

export interface DeliveryLog {
  id: string;
  message_id: string;
  provider_id?: string;
  status: string;
  provider?: string;
  provider_message_id?: string;
  response_code?: number;
  retry_count: number;
  last_error?: string;
  duration_ms?: number;
  attempt_number: number;
  created_at?: string;
  delivered_at?: string;
}

export interface MessageDetail {
  message: Message;
  delivery_logs: DeliveryLog[];
}

export interface PaginatedMessages {
  data: Message[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface ActivityLog {
  id: string;
  group_id: string;
  actor_id?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  changes?: Record<string, unknown>;
  comment?: string;
  ip_address?: string;
  created_at: string;
}

export interface ProviderAccess {
  provider_id: string;
  group_id: string;
  granted_at: string;
  granted_by?: string;
}

export interface ApiError {
  error: string;
  details?: string[];
}
