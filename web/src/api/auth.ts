import api from './client';
import type { LoginRequest, LoginResponse, MeResponse } from '../types/api';

export async function login(req: LoginRequest): Promise<LoginResponse> {
  const { data } = await api.post<LoginResponse>('/auth/login', req);
  return data;
}

export async function refreshTokens(refresh_token: string): Promise<LoginResponse> {
  const { data } = await api.post<LoginResponse>('/auth/refresh', { refresh_token });
  return data;
}

export async function logout(refresh_token: string): Promise<void> {
  await api.post('/auth/logout', { refresh_token });
}

export async function switchGroup(group_id: string): Promise<LoginResponse> {
  const { data } = await api.post<LoginResponse>('/auth/switch-group', { group_id });
  return data;
}

export async function getMe(): Promise<MeResponse> {
  const { data } = await api.get<MeResponse>('/auth/me');
  return data;
}

export async function changePassword(current_password: string, new_password: string): Promise<void> {
  await api.patch('/users/me/password', { current_password, new_password });
}
