import { useQuery } from '@tanstack/react-query';
import {
  PageSection, Title, Spinner,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchActivityLogs } from '../api/resources';
import { useAuth } from '../context/AuthContext';

export default function ActivityLogPage() {
  const { me } = useAuth();
  const groupId = me?.current_group.group_id;

  const { data: logs, isLoading } = useQuery({
    queryKey: ['activity-logs', groupId],
    queryFn: () => fetchActivityLogs(groupId!, 100),
    enabled: !!groupId,
    refetchInterval: 30000,
  });

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <Title headingLevel="h1" size="lg" className="page-title">
        Activity Log
      </Title>

      <Table aria-label="Activity logs">
        <Thead>
          <Tr>
            <Th>Action</Th>
            <Th>Resource</Th>
            <Th>Actor</Th>
            <Th>IP Address</Th>
            <Th>Comment</Th>
            <Th>Time</Th>
          </Tr>
        </Thead>
        <Tbody>
          {logs?.map((a) => (
            <Tr key={a.id}>
              <Td>{a.action}</Td>
              <Td>{a.resource_type}{a.resource_id ? ` (${a.resource_id.slice(0, 8)}...)` : ''}</Td>
              <Td>{a.actor_id ? `${a.actor_id.slice(0, 8)}...` : 'System'}</Td>
              <Td>{a.ip_address || '-'}</Td>
              <Td>{a.comment || '-'}</Td>
              <Td>{new Date(a.created_at).toLocaleString()}</Td>
            </Tr>
          ))}
        </Tbody>
      </Table>
    </PageSection>
  );
}
