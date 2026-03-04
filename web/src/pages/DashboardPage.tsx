import { useQuery } from '@tanstack/react-query';
import {
  PageSection, Title, Grid, GridItem, Card, CardTitle, CardBody,
  Spinner, EmptyState, EmptyStateBody,
} from '@patternfly/react-core';
import { fetchDashboardStats, fetchTimeSeries, fetchUsageByUser, fetchUsageByProvider } from '../api/resources';
import type { TimeSeriesPoint, UsageByUser, UsageByProvider } from '../types/api';

function aggregateTimeSeries(points: TimeSeriesPoint[]) {
  const byDay = new Map<string, { sent: number; failed: number; total: number }>();
  for (const p of points) {
    const entry = byDay.get(p.day) || { sent: 0, failed: 0, total: 0 };
    entry.total += p.count;
    if (p.status === 'sent') entry.sent += p.count;
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
    if (p.status === 'sent') entry.sent += p.count;
    if (p.status === 'failed') entry.failed += p.count;
    byUser.set(p.user_id, entry);
  }
  return Array.from(byUser.entries()).map(([user_id, counts]) => ({
    user_id, ...counts, total: counts.sent + counts.failed,
  }));
}

function aggregateByProvider(points: UsageByProvider[]) {
  const byProvider = new Map<string, { sent: number; failed: number }>();
  for (const p of points) {
    const entry = byProvider.get(p.provider) || { sent: 0, failed: 0 };
    if (p.status === 'sent') entry.sent += p.count;
    if (p.status === 'failed') entry.failed += p.count;
    byProvider.set(p.provider, entry);
  }
  return Array.from(byProvider.entries()).map(([provider, counts]) => ({
    provider, ...counts, total: counts.sent + counts.failed,
  }));
}

export default function DashboardPage() {
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: () => fetchDashboardStats(),
    refetchInterval: 15000,
  });

  const { data: timeSeries } = useQuery({
    queryKey: ['dashboard-timeseries'],
    queryFn: () => fetchTimeSeries(),
    refetchInterval: 15000,
  });

  const { data: usageByUser } = useQuery({
    queryKey: ['dashboard-usage-by-user'],
    queryFn: () => fetchUsageByUser(),
    refetchInterval: 15000,
  });

  const { data: usageByProvider } = useQuery({
    queryKey: ['dashboard-usage-by-provider'],
    queryFn: () => fetchUsageByProvider(),
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
  const byProvider = usageByProvider ? aggregateByProvider(usageByProvider) : [];

  return (
    <PageSection>
      <Title headingLevel="h1" size="lg" style={{ marginBottom: '1rem' }}>
        Dashboard
      </Title>

      <Grid hasGutter>
        <GridItem span={3}>
          <Card>
            <CardTitle>Total Messages</CardTitle>
            <CardBody>
              <span style={{ fontSize: '2rem', fontWeight: 'bold' }}>
                {stats.total_messages.toLocaleString()}
              </span>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem span={3}>
          <Card>
            <CardTitle>Success Rate</CardTitle>
            <CardBody>
              <span style={{ fontSize: '2rem', fontWeight: 'bold', color: stats.success_rate >= 95 ? '#3E8635' : stats.success_rate >= 80 ? '#F0AB00' : '#C9190B' }}>
                {stats.success_rate.toFixed(1)}%
              </span>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem span={3}>
          <Card>
            <CardTitle>Sent</CardTitle>
            <CardBody>
              <span style={{ fontSize: '2rem', fontWeight: 'bold', color: '#3E8635' }}>
                {(stats.status_counts['sent'] || 0).toLocaleString()}
              </span>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem span={3}>
          <Card>
            <CardTitle>Failed</CardTitle>
            <CardBody>
              <span style={{ fontSize: '2rem', fontWeight: 'bold', color: '#C9190B' }}>
                {(stats.status_counts['failed'] || 0).toLocaleString()}
              </span>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem span={12}>
          <Card>
            <CardTitle>Daily Delivery Trend (Last 30 Days)</CardTitle>
            <CardBody>
              {daily.length === 0 ? (
                <p>No delivery data in this period</p>
              ) : (
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                    <thead>
                      <tr>
                        <th style={{ textAlign: 'left', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Date</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Total</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Sent</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Failed</th>
                      </tr>
                    </thead>
                    <tbody>
                      {daily.slice(-14).map((row) => (
                        <tr key={row.day}>
                          <td style={{ padding: '0.5rem', borderBottom: '1px solid #eee' }}>{row.day}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee' }}>{row.total}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee', color: '#3E8635' }}>{row.sent}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee', color: '#C9190B' }}>{row.failed}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardBody>
          </Card>
        </GridItem>

        <GridItem span={6}>
          <Card>
            <CardTitle>Usage by User</CardTitle>
            <CardBody>
              {byUser.length === 0 ? (
                <p>No user usage data</p>
              ) : (
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                    <thead>
                      <tr>
                        <th style={{ textAlign: 'left', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>User</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Sent</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Failed</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Total</th>
                      </tr>
                    </thead>
                    <tbody>
                      {byUser.map((row) => (
                        <tr key={row.user_id}>
                          <td style={{ padding: '0.5rem', borderBottom: '1px solid #eee' }}>{row.user_id.slice(0, 8)}...</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee', color: '#3E8635' }}>{row.sent}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee', color: '#C9190B' }}>{row.failed}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee' }}>{row.total}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardBody>
          </Card>
        </GridItem>

        <GridItem span={6}>
          <Card>
            <CardTitle>Usage by Provider</CardTitle>
            <CardBody>
              {byProvider.length === 0 ? (
                <p>No provider usage data</p>
              ) : (
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                    <thead>
                      <tr>
                        <th style={{ textAlign: 'left', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Provider</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Sent</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Failed</th>
                        <th style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '2px solid #d2d2d2' }}>Total</th>
                      </tr>
                    </thead>
                    <tbody>
                      {byProvider.map((row) => (
                        <tr key={row.provider}>
                          <td style={{ padding: '0.5rem', borderBottom: '1px solid #eee' }}>{row.provider}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee', color: '#3E8635' }}>{row.sent}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee', color: '#C9190B' }}>{row.failed}</td>
                          <td style={{ textAlign: 'right', padding: '0.5rem', borderBottom: '1px solid #eee' }}>{row.total}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardBody>
          </Card>
        </GridItem>
      </Grid>
    </PageSection>
  );
}
