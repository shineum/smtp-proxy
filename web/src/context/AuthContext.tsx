import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import { setTokens, clearTokens, setOnTokenRefreshFailed, getRefreshToken } from '../api/client';
import { login as apiLogin, getMe, logout as apiLogout, switchGroup as apiSwitchGroup } from '../api/auth';
import type { MeResponse } from '../types/api';

interface AuthState {
  isAuthenticated: boolean;
  isLoading: boolean;
  me: MeResponse | null;
}

interface AuthContextType extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  switchGroup: (groupId: string) => Promise<void>;
  refreshProfile: () => Promise<void>;
  isSystemAdmin: boolean;
  isAdmin: boolean;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    isAuthenticated: false,
    isLoading: true,
    me: null,
  });

  const handleLogout = useCallback(async () => {
    const rt = getRefreshToken();
    if (rt) {
      try { await apiLogout(rt); } catch { /* best effort */ }
    }
    clearTokens();
    setState({ isAuthenticated: false, isLoading: false, me: null });
  }, []);

  // Set up token refresh failure handler
  useEffect(() => {
    setOnTokenRefreshFailed(() => {
      setState({ isAuthenticated: false, isLoading: false, me: null });
    });
  }, []);

  // Try to restore session on mount (from sessionStorage)
  useEffect(() => {
    const stored = sessionStorage.getItem('smtp_proxy_tokens');
    if (stored) {
      try {
        const { access_token, refresh_token } = JSON.parse(stored);
        setTokens(access_token, refresh_token);
        getMe()
          .then((me) => setState({ isAuthenticated: true, isLoading: false, me }))
          .catch(() => {
            clearTokens();
            sessionStorage.removeItem('smtp_proxy_tokens');
            setState({ isAuthenticated: false, isLoading: false, me: null });
          });
      } catch {
        sessionStorage.removeItem('smtp_proxy_tokens');
        setState({ isAuthenticated: false, isLoading: false, me: null });
      }
    } else {
      setState((s) => ({ ...s, isLoading: false }));
    }
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const res = await apiLogin({ email, password });
    setTokens(res.access_token, res.refresh_token);
    sessionStorage.setItem('smtp_proxy_tokens', JSON.stringify({
      access_token: res.access_token,
      refresh_token: res.refresh_token,
    }));
    const me = await getMe();
    setState({ isAuthenticated: true, isLoading: false, me });
  }, []);

  const logout = useCallback(async () => {
    await handleLogout();
    sessionStorage.removeItem('smtp_proxy_tokens');
  }, [handleLogout]);

  const switchGroupFn = useCallback(async (groupId: string) => {
    const res = await apiSwitchGroup(groupId);
    setTokens(res.access_token, res.refresh_token);
    sessionStorage.setItem('smtp_proxy_tokens', JSON.stringify({
      access_token: res.access_token,
      refresh_token: res.refresh_token,
    }));
    const me = await getMe();
    setState({ isAuthenticated: true, isLoading: false, me });
  }, []);

  const refreshProfile = useCallback(async () => {
    const me = await getMe();
    setState((s) => ({ ...s, me }));
  }, []);

  const isSystemAdmin = state.me?.current_group?.group_type === 'system';
  const currentRole = state.me?.current_group?.role;
  const isAdmin = isSystemAdmin || currentRole === 'admin' || currentRole === 'owner';

  return (
    <AuthContext.Provider value={{
      ...state,
      login,
      logout,
      switchGroup: switchGroupFn,
      refreshProfile,
      isSystemAdmin,
      isAdmin,
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used within AuthProvider');
  return context;
}
