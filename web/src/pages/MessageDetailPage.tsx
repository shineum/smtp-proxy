import { useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Card, CardTitle, CardBody,
  DescriptionList, DescriptionListGroup, DescriptionListTerm, DescriptionListDescription,
  Label, Spinner, Button,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchMessage, reprocessDLQ } from '../api/resources';

const STATUS_COLORS: Record<string, 'blue' | 'green' | 'red' | 'orange' | 'grey'> = {
  queued: 'blue',
  sent: 'green',
  failed: 'red',
  processing: 'orange',
  delivered: 'green',
  bounced: 'red',
};

export default function MessageDetailPage() {
  const { id } = useParams<{ id: string }>();

  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ['message', id],
    queryFn: () => fetchMessage(id!),
    enabled: !!id,
  });

  const reprocessMutation = useMutation({
    mutationFn: () => reprocessDLQ([id!]),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['message', id] }),
  });

  if (isLoading || !data) return <PageSection><Spinner size="xl" /></PageSection>;

  const { message, delivery_logs } = data;

  return (
    <PageSection>
      <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '1rem' }}>
        <Title headingLevel="h1" size="lg">
          Message: {message.subject || '(no subject)'}
        </Title>
        {message.status === 'failed' && (
          <Button
            variant="secondary"
            size="sm"
            onClick={() => reprocessMutation.mutate()}
            isDisabled={reprocessMutation.isPending}
          >
            {reprocessMutation.isPending ? 'Reprocessing...' : 'Reprocess'}
          </Button>
        )}
      </div>
      {reprocessMutation.isSuccess && (
        <p style={{ color: '#3E8635', marginBottom: '1rem' }}>Message queued for reprocessing.</p>
      )}
      {reprocessMutation.isError && (
        <p style={{ color: '#C9190B', marginBottom: '1rem' }}>
          Failed to reprocess. DLQ may not be enabled.
        </p>
      )}

      <Card style={{ marginBottom: '1rem' }}>
        <CardTitle>Details</CardTitle>
        <CardBody>
          <DescriptionList>
            <DescriptionListGroup>
              <DescriptionListTerm>ID</DescriptionListTerm>
              <DescriptionListDescription>{message.id}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>From</DescriptionListTerm>
              <DescriptionListDescription>{message.sender}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>To</DescriptionListTerm>
              <DescriptionListDescription>{message.recipients?.join(', ') || '-'}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Status</DescriptionListTerm>
              <DescriptionListDescription>
                <Label color={STATUS_COLORS[message.status] || 'grey'}>{message.status}</Label>
              </DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Enqueued</DescriptionListTerm>
              <DescriptionListDescription>
                {message.enqueued_at ? new Date(message.enqueued_at).toLocaleString() : '-'}
              </DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Processed</DescriptionListTerm>
              <DescriptionListDescription>
                {message.processed_at ? new Date(message.processed_at).toLocaleString() : '-'}
              </DescriptionListDescription>
            </DescriptionListGroup>
          </DescriptionList>
        </CardBody>
      </Card>

      <Card>
        <CardTitle>Delivery Timeline</CardTitle>
        <CardBody>
          {delivery_logs.length === 0 ? (
            <p>No delivery attempts yet</p>
          ) : (
            <Table aria-label="Delivery logs">
              <Thead>
                <Tr>
                  <Th>Attempt</Th>
                  <Th>Provider</Th>
                  <Th>Status</Th>
                  <Th>Response</Th>
                  <Th>Duration</Th>
                  <Th>Time</Th>
                  <Th>Error</Th>
                </Tr>
              </Thead>
              <Tbody>
                {delivery_logs.map((dl) => (
                  <Tr key={dl.id}>
                    <Td>#{dl.attempt_number}</Td>
                    <Td>{dl.provider || '-'}</Td>
                    <Td><Label color={STATUS_COLORS[dl.status] || 'grey'}>{dl.status}</Label></Td>
                    <Td>{dl.response_code || '-'}</Td>
                    <Td>{dl.duration_ms != null ? `${dl.duration_ms}ms` : '-'}</Td>
                    <Td>{dl.created_at ? new Date(dl.created_at).toLocaleString() : '-'}</Td>
                    <Td style={{ color: '#C9190B', maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {dl.last_error || '-'}
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          )}
        </CardBody>
      </Card>
    </PageSection>
  );
}
