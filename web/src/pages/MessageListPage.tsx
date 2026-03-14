import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  PageSection, Title, Spinner, Label,
  Pagination, FormSelect, FormSelectOption,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchMessages } from '../api/resources';

const STATUS_COLORS: Record<string, 'blue' | 'green' | 'red' | 'orange' | 'grey'> = {
  queued: 'blue',
  sent: 'green',
  failed: 'red',
  processing: 'orange',
};

export default function MessageListPage() {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [statusFilter, setStatusFilter] = useState('');

  const { data, isLoading } = useQuery({
    queryKey: ['messages', page, pageSize, statusFilter],
    queryFn: () => fetchMessages(page, pageSize, statusFilter || undefined),
    refetchInterval: 30000,
  });

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div className="page-header">
        <Title headingLevel="h1" size="lg">Messages</Title>
        <FormSelect
          value={statusFilter}
          onChange={(_e, v) => { setStatusFilter(v); setPage(1); }}
          style={{ width: '200px' }}
          aria-label="Status filter"
        >
          <FormSelectOption value="" label="All Statuses" />
          <FormSelectOption value="queued" label="Queued" />
          <FormSelectOption value="processing" label="Processing" />
          <FormSelectOption value="sent" label="Sent" />
          <FormSelectOption value="failed" label="Failed" />
        </FormSelect>
      </div>

      <Table aria-label="Messages table">
        <Thead>
          <Tr>
            <Th>Subject</Th>
            <Th>From</Th>
            <Th>To</Th>
            <Th>Status</Th>
            <Th>Enqueued</Th>
          </Tr>
        </Thead>
        <Tbody>
          {data?.data.map((m) => (
            <Tr key={m.id} isClickable onRowClick={() => navigate(`/messages/${m.id}`)}>
              <Td>{m.subject || '(no subject)'}</Td>
              <Td>{m.sender}</Td>
              <Td>{m.recipients?.join(', ') || '-'}</Td>
              <Td><Label color={STATUS_COLORS[m.status] || 'grey'}>{m.status}</Label></Td>
              <Td>{m.enqueued_at ? new Date(m.enqueued_at).toLocaleString() : '-'}</Td>
            </Tr>
          ))}
        </Tbody>
      </Table>

      {data && (
        <Pagination
          itemCount={data.total}
          perPage={pageSize}
          page={page}
          onSetPage={(_e, p) => setPage(p)}
          onPerPageSelect={(_e, ps) => { setPageSize(ps); setPage(1); }}
          className="pagination-footer"
        />
      )}
    </PageSection>
  );
}
