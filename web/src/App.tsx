import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider, useAuth } from './context/AuthContext';
import Layout from './components/Layout';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import GroupListPage from './pages/GroupListPage';
import GroupDetailPage from './pages/GroupDetailPage';
import UserListPage from './pages/UserListPage';
import UserDetailPage from './pages/UserDetailPage';
import ProviderListPage from './pages/ProviderListPage';
import ProviderFormPage from './pages/ProviderFormPage';
import RoutingRuleListPage from './pages/RoutingRuleListPage';
import RoutingRuleFormPage from './pages/RoutingRuleFormPage';
import MessageListPage from './pages/MessageListPage';
import MessageDetailPage from './pages/MessageDetailPage';
import ActivityLogPage from './pages/ActivityLogPage';
import SettingsPage from './pages/SettingsPage';
import DomainRateLimitPage from './pages/DomainRateLimitPage';
import type { ReactNode } from 'react';
import { Spinner, PageSection } from '@patternfly/react-core';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RedirectIfAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;
  if (isAuthenticated) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function RequireAdmin({ children }: { children: ReactNode }) {
  const { isAdmin, isLoading } = useAuth();
  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;
  if (!isAdmin) return <Navigate to="/" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<RedirectIfAuth><LoginPage /></RedirectIfAuth>} />
            <Route element={<RequireAuth><Layout /></RequireAuth>}>
              <Route path="/" element={<DashboardPage />} />
              <Route path="/groups" element={<GroupListPage />} />
              <Route path="/groups/:id" element={<GroupDetailPage />} />
              <Route path="/users" element={<RequireAdmin><UserListPage /></RequireAdmin>} />
              <Route path="/users/:id" element={<RequireAdmin><UserDetailPage /></RequireAdmin>} />
              <Route path="/providers" element={<ProviderListPage />} />
              <Route path="/providers/new" element={<ProviderFormPage />} />
              <Route path="/providers/:id" element={<ProviderFormPage />} />
              <Route path="/routing-rules" element={<RoutingRuleListPage />} />
              <Route path="/routing-rules/new" element={<RoutingRuleFormPage />} />
              <Route path="/routing-rules/:id" element={<RoutingRuleFormPage />} />
              <Route path="/messages" element={<MessageListPage />} />
              <Route path="/messages/:id" element={<MessageDetailPage />} />
              <Route path="/domain-rate-limits" element={<DomainRateLimitPage />} />
              <Route path="/activity" element={<ActivityLogPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Route>
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
