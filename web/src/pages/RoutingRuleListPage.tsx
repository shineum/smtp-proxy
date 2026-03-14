import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Button, Spinner, Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchRoutingRules, deleteRoutingRule } from '../api/resources';

export default function RoutingRuleListPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: rules, isLoading } = useQuery({
    queryKey: ['routing-rules'],
    queryFn: fetchRoutingRules,
  });

  const deleteMutation = useMutation({
    mutationFn: deleteRoutingRule,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['routing-rules'] }),
  });

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div className="page-header">
        <Title headingLevel="h1" size="lg">Routing Rules</Title>
        <Button onClick={() => navigate('/routing-rules/new')}>Add Rule</Button>
      </div>

      <Table aria-label="Routing rules table">
        <Thead>
          <Tr>
            <Th>Priority</Th>
            <Th>Conditions</Th>
            <Th>Provider</Th>
            <Th>Enabled</Th>
            <Th>Created</Th>
            <Th>Actions</Th>
          </Tr>
        </Thead>
        <Tbody>
          {rules?.map((r) => (
            <Tr key={r.id} isClickable onRowClick={() => navigate(`/routing-rules/${r.id}`)}>
              <Td>{r.priority}</Td>
              <Td><code>{JSON.stringify(r.conditions)}</code></Td>
              <Td>{r.provider_id.slice(0, 8)}...</Td>
              <Td><Label color={r.enabled ? 'green' : 'grey'}>{r.enabled ? 'Yes' : 'No'}</Label></Td>
              <Td>{new Date(r.created_at).toLocaleDateString()}</Td>
              <Td>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={(e) => { e.stopPropagation(); if (confirm('Delete this routing rule?')) deleteMutation.mutate(r.id); }}
                >
                  Delete
                </Button>
              </Td>
            </Tr>
          ))}
        </Tbody>
      </Table>
    </PageSection>
  );
}
