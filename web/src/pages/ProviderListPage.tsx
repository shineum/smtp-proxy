import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Button, Spinner, Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchProviders, deleteProvider, fetchProviderHealth } from '../api/resources';
import type { ProviderHealth } from '../types/api';
import { useState, useEffect } from 'react';

export default function ProviderListPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [healthMap, setHealthMap] = useState<Record<string, ProviderHealth>>({});

  const { data: providers, isLoading } = useQuery({
    queryKey: ['providers'],
    queryFn: fetchProviders,
  });

  const deleteMutation = useMutation({
    mutationFn: deleteProvider,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['providers'] }),
  });

  useEffect(() => {
    if (!providers) return;
    providers.forEach((p) => {
      fetchProviderHealth(p.id).then((h) => {
        setHealthMap((prev) => ({ ...prev, [p.id]: h }));
      }).catch(() => {});
    });
  }, [providers]);

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <Title headingLevel="h1" size="lg">ESP Providers</Title>
        <Button onClick={() => navigate('/providers/new')}>Add Provider</Button>
      </div>

      <Table aria-label="Providers table">
        <Thead>
          <Tr>
            <Th>Name</Th>
            <Th>Type</Th>
            <Th>Enabled</Th>
            <Th>Health (24h)</Th>
            <Th>Created</Th>
            <Th>Actions</Th>
          </Tr>
        </Thead>
        <Tbody>
          {providers?.map((p) => {
            const health = healthMap[p.id];
            return (
              <Tr key={p.id} isClickable onRowClick={() => navigate(`/providers/${p.id}`)}>
                <Td>{p.name}</Td>
                <Td><Label>{p.provider_type}</Label></Td>
                <Td><Label color={p.enabled ? 'green' : 'grey'}>{p.enabled ? 'Yes' : 'No'}</Label></Td>
                <Td>
                  {health ? (
                    <span>
                      <span style={{ color: '#3E8635' }}>{health.sent_24h} sent</span>
                      {' / '}
                      <span style={{ color: '#C9190B' }}>{health.failed_24h} failed</span>
                    </span>
                  ) : '-'}
                </Td>
                <Td>{new Date(p.created_at).toLocaleDateString()}</Td>
                <Td>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={(e) => { e.stopPropagation(); if (confirm(`Delete provider "${p.name}"?`)) deleteMutation.mutate(p.id); }}
                  >
                    Delete
                  </Button>
                </Td>
              </Tr>
            );
          })}
        </Tbody>
      </Table>
    </PageSection>
  );
}
