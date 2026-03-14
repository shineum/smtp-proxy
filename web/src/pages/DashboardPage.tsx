import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  PageSection, Title, Grid, GridItem, Card, CardTitle, CardBody,
  Spinner, EmptyState, EmptyStateBody,
  Select, SelectOption, SelectList,
  MenuToggle, Badge,
} from '@patternfly/react-core';
import type { MenuToggleElement } from '@patternfly/react-core';
import {
  Table, Thead, Tbody, Tr, Th, Td,
} from '@patternfly/react-table';
import {
  EnvelopeIcon,
  CheckCircleIcon,
  PaperPlaneIcon,
  ExclamationCircleIcon,
} from '@patternfly/react-icons';
import { fetchDashboardStats, fetchTimeSeries, fetchUsageByUser, fetchUsageByGroup, fetchUsageByProvider, fetchGroups } from '../api/resources';
import { useAuth } from '../context/AuthContext';
import type { TimeSeriesPoint, UsageByUser, UsageByGroup, UsageByProvider } from '../types/api';

function aggregateTimeSeries(points: TimeSeriesPoint[]) {
  const byDay = new Map<string, { sent: number; failed: number; total: number }>();
  for (const p of points) {
    const entry = byDay.get(p.day) || { sent: 0, failed: 0, total: 0 };
    entry.total += p.count;
    if (p.status === 'delivered') entry.sent += p.count;
    if (p.status === 'failed') entry.failed += p.count;
    byDay.set(p.day, entry);
  }
  return Array.from(byDay.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([day, counts]) => ({ day, ...counts }));
}

function aggregateByUser(points: UsageByUser[]) {
  const byUser = new Map<string, { sent: number; failed: number }>();
  for (const p of points) {
    const entry = byUser.get(p.user_id) || { sent: 0, failed: 0 };
    if (p.status === 'delivered') entry.sent += p.count;
    if (p.status === 'failed') entry.failed += p.count;
    byUser.set(p.user_id, entry);
  }
  return Array.from(byUser.entries()).map(([user_id, counts]) => ({
    user_id, ...counts, total: counts.sent + counts.failed,
  }));
}

function aggregateByGroup(points: UsageByGroup[]) {
  const byGroup = new Map<string, { group_name: string; sent: number; failed: number }>();
  for (const p of points) {
    const entry = byGroup.get(p.group_id) || { group_name: p.group_name, sent: 0, failed: 0 };
    if (p.status === 'delivered') entry.sent += p.count;
    if (p.status === 'failed') entry.failed += p.count;
    byGroup.set(p.group_id, entry);
  }
  return Array.from(byGroup.entries()).map(([group_id, { group_name, sent, failed }]) => ({
    group_id, group_name, sent, failed, total: sent + failed,
  }));
}

function aggregateByProvider(points: UsageByProvider[]) {
  const byProvider = new Map<string, { sent: number; failed: number }>();
  for (const p of points) {
    const entry = byProvider.get(p.provider) || { sent: 0, failed: 0 };
    if (p.status === 'delivered') entry.sent += p.count;
    if (p.status === 'failed') entry.failed += p.count;
    byProvider.set(p.provider, entry);
  }
  return Array.from(byProvider.entries()).map(([provider, counts]) => ({
    provider, ...counts, total: counts.sent + counts.failed,
  }));
}

export default function DashboardPage() {
  const { isSystemAdmin } = useAuth();
  const [selectedGroupIds, setSelectedGroupIds] = useState<string[]>([]);
  const [isFilterOpen, setIsFilterOpen] = useState(false);

  const { data: groups } = useQuery({
    queryKey: ['groups'],
    queryFn: fetchGroups,
    enabled: isSystemAdmin,
  });

  const groupIdParam = selectedGroupIds.length > 0 ? selectedGroupIds.join(',') : undefined;

  const onGroupSelect = (_event: React.MouseEvent | undefined, value: string | number | undefined) => {
    const id = String(value);
    setSelectedGroupIds(prev =>
      prev.includes(id) ? prev.filter(g => g !== id) : [...prev, id]
    );
  };

  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['dashboard-stats', selectedGroupIds],
    queryFn: () => fetchDashboardStats(undefined, undefined, groupIdParam),
    refetchInterval: 15000,
  });

  const { data: timeSeries } = useQuery({
    queryKey: ['dashboard-timeseries', selectedGroupIds],
    queryFn: () => fetchTimeSeries(undefined, undefined, groupIdParam),
    refetchInterval: 15000,
  });

  const { data: usageByUser } = useQuery({
    queryKey: ['dashboard-usage-by-user', selectedGroupIds],
    queryFn: () => fetchUsageByUser(undefined, undefined, groupIdParam),
    refetchInterval: 15000,
    enabled: !isSystemAdmin,
  });

  const { data: usageByGroup } = useQuery({
    queryKey: ['dashboard-usage-by-group'],
    queryFn: () => fetchUsageByGroup(),
    refetchInterval: 15000,
    enabled: isSystemAdmin,
  });

  const { data: usageByProvider } = useQuery({
    queryKey: ['dashboard-usage-by-provider', selectedGroupIds],
    queryFn: () => fetchUsageByProvider(undefined, undefined, groupIdParam),
    refetchInterval: 15000,
  });

  if (statsLoading) {
    return (
      <PageSection>
        <Spinner size="xl" />
      </PageSection>
    );
  }

  if (!stats) {
    return (
      <PageSection>
        <EmptyState>
          <EmptyStateBody>No data available</EmptyStateBody>
        </EmptyState>
      </PageSection>
    );
  }

  const daily = timeSeries ? aggregateTimeSeries(timeSeries) : [];
  const byUser = usageByUser ? aggregateByUser(usageByUser) : [];
  const byGroup = usageByGroup ? aggregateByGroup(usageByGroup) : [];
  const byProvider = usageByProvider ? aggregateByProvider(usageByProvider) : [];

  const successRateColor = stats.success_rate >= 95
    ? 'var(--color-success)'
    : stats.success_rate >= 80
    ? 'var(--color-warning)'
    : 'var(--color-danger)';

  return (
    <PageSection>
      <Title headingLevel="h1" size="lg" className="page-title">
        Dashboard
      </Title>

      {isSystemAdmin && (
        <div className="filter-bar">
          <Select
            role="menu"
            id="group-filter"
            isOpen={isFilterOpen}
            selected={selectedGroupIds}
            onSelect={onGroupSelect}
            onOpenChange={setIsFilterOpen}
            toggle={(toggleRef: React.Ref<MenuToggleElement>) => (
              <MenuToggle
                ref={toggleRef}
                onClick={() => setIsFilterOpen(prev => !prev)}
                isExpanded={isFilterOpen}
                style={{ minWidth: '200px' }}
              >
                Filter by Group
                {selectedGroupIds.length > 0 && (
                  <>
                    {' '}
                    <Badge isRead>{selectedGroupIds.length}</Badge>
                  </>
                )}
              </MenuToggle>
            )}
          >
            <SelectList>
              {groups?.map((group) => (
                <SelectOption
                  key={group.id}
                  value={group.id}
                  hasCheckbox
                  isSelected={selectedGroupIds.includes(group.id)}
                >
                  {group.display_name || group.name}
                </SelectOption>
              ))}
            </SelectList>
          </Select>
        </div>
      )}

      {/* Stat Cards - flex wrap with fixed width */}
      <div className="stat-cards-grid">
        <Card className="stat-card stat-card--total">
          <CardTitle>
            <span className="stat-card__title">
              <EnvelopeIcon className="stat-card__icon" />
              Total Messages
            </span>
          </CardTitle>
          <CardBody>
            <span className="stat-card__value">
              {stats.total_messages.toLocaleString()}
            </span>
          </CardBody>
        </Card>

        <Card className="stat-card stat-card--success">
          <CardTitle>
            <span className="stat-card__title">
              <CheckCircleIcon className="stat-card__icon" />
              Success Rate
            </span>
          </CardTitle>
          <CardBody>
            <span className="stat-card__value" style={{ color: successRateColor }}>
              {stats.success_rate.toFixed(1)}%
            </span>
          </CardBody>
        </Card>

        <Card className="stat-card stat-card--delivered">
          <CardTitle>
            <span className="stat-card__title">
              <PaperPlaneIcon className="stat-card__icon" />
              Delivered
            </span>
          </CardTitle>
          <CardBody>
            <span className="stat-card__value stat-card__value--delivered">
              {(stats.status_counts['delivered'] || 0).toLocaleString()}
            </span>
          </CardBody>
        </Card>

        <Card className="stat-card stat-card--failed">
          <CardTitle>
            <span className="stat-card__title">
              <ExclamationCircleIcon className="stat-card__icon" />
              Failed
            </span>
          </CardTitle>
          <CardBody>
            <span className="stat-card__value stat-card__value--failed">
              {(stats.status_counts['failed'] || 0).toLocaleString()}
            </span>
          </CardBody>
        </Card>
      </div>

      <Grid hasGutter>
        {/* Daily Trend Table */}
        <GridItem span={12}>
          <Card>
            <CardTitle>Daily Delivery Trend (Last 14 Days)</CardTitle>
            <CardBody>
              {daily.length === 0 ? (
                <p>No delivery data in this period</p>
              ) : (
                <Table aria-label="Daily delivery trend" variant="compact">
                  <Thead>
                    <Tr>
                      <Th>Date</Th>
                      <Th modifier="wrap" style={{ textAlign: 'right' }}>Total</Th>
                      <Th modifier="wrap" style={{ textAlign: 'right' }}>Sent</Th>
                      <Th modifier="wrap" style={{ textAlign: 'right' }}>Failed</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {daily.slice(-14).map((row) => (
                      <Tr key={row.day}>
                        <Td dataLabel="Date">{row.day}</Td>
                        <Td dataLabel="Total" style={{ textAlign: 'right' }}>{row.total}</Td>
                        <Td dataLabel="Sent" style={{ textAlign: 'right', color: 'var(--color-success)' }}>{row.sent}</Td>
                        <Td dataLabel="Failed" style={{ textAlign: 'right', color: 'var(--color-danger)' }}>{row.failed}</Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              )}
            </CardBody>
          </Card>
        </GridItem>

        {/* Usage by Group or User */}
        <GridItem span={6}>
          {isSystemAdmin ? (
            <Card>
              <CardTitle>Usage by Group</CardTitle>
              <CardBody>
                {byGroup.length === 0 ? (
                  <p>No group usage data</p>
                ) : (
                  <Table aria-label="Usage by group" variant="compact">
                    <Thead>
                      <Tr>
                        <Th>Group</Th>
                        <Th modifier="wrap" style={{ textAlign: 'right' }}>Sent</Th>
                        <Th modifier="wrap" style={{ textAlign: 'right' }}>Failed</Th>
                        <Th modifier="wrap" style={{ textAlign: 'right' }}>Total</Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {byGroup.map((row) => (
                        <Tr key={row.group_id}>
                          <Td dataLabel="Group">{row.group_name}</Td>
                          <Td dataLabel="Sent" style={{ textAlign: 'right', color: 'var(--color-success)' }}>{row.sent}</Td>
                          <Td dataLabel="Failed" style={{ textAlign: 'right', color: 'var(--color-danger)' }}>{row.failed}</Td>
                          <Td dataLabel="Total" style={{ textAlign: 'right' }}>{row.total}</Td>
                        </Tr>
                      ))}
                    </Tbody>
                  </Table>
                )}
              </CardBody>
            </Card>
          ) : (
            <Card>
              <CardTitle>Usage by User</CardTitle>
              <CardBody>
                {byUser.length === 0 ? (
                  <p>No user usage data</p>
                ) : (
                  <Table aria-label="Usage by user" variant="compact">
                    <Thead>
                      <Tr>
                        <Th>User</Th>
                        <Th modifier="wrap" style={{ textAlign: 'right' }}>Sent</Th>
                        <Th modifier="wrap" style={{ textAlign: 'right' }}>Failed</Th>
                        <Th modifier="wrap" style={{ textAlign: 'right' }}>Total</Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {byUser.map((row) => (
                        <Tr key={row.user_id}>
                          <Td dataLabel="User" className="mono">{row.user_id.slice(0, 8)}...</Td>
                          <Td dataLabel="Sent" style={{ textAlign: 'right', color: 'var(--color-success)' }}>{row.sent}</Td>
                          <Td dataLabel="Failed" style={{ textAlign: 'right', color: 'var(--color-danger)' }}>{row.failed}</Td>
                          <Td dataLabel="Total" style={{ textAlign: 'right' }}>{row.total}</Td>
                        </Tr>
                      ))}
                    </Tbody>
                  </Table>
                )}
              </CardBody>
            </Card>
          )}
        </GridItem>

        {/* Usage by Provider */}
        <GridItem span={6}>
          <Card>
            <CardTitle>Usage by Provider</CardTitle>
            <CardBody>
              {byProvider.length === 0 ? (
                <p>No provider usage data</p>
              ) : (
                <Table aria-label="Usage by provider" variant="compact">
                  <Thead>
                    <Tr>
                      <Th>Provider</Th>
                      <Th modifier="wrap" style={{ textAlign: 'right' }}>Sent</Th>
                      <Th modifier="wrap" style={{ textAlign: 'right' }}>Failed</Th>
                      <Th modifier="wrap" style={{ textAlign: 'right' }}>Total</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {byProvider.map((row) => (
                      <Tr key={row.provider}>
                        <Td dataLabel="Provider">{row.provider}</Td>
                        <Td dataLabel="Sent" style={{ textAlign: 'right', color: 'var(--color-success)' }}>{row.sent}</Td>
                        <Td dataLabel="Failed" style={{ textAlign: 'right', color: 'var(--color-danger)' }}>{row.failed}</Td>
                        <Td dataLabel="Total" style={{ textAlign: 'right' }}>{row.total}</Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              )}
            </CardBody>
          </Card>
        </GridItem>
      </Grid>
    </PageSection>
  );
}
