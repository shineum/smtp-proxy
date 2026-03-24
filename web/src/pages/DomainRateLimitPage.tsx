import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Card, CardBody,
  Button, Modal, ModalVariant, Form, FormGroup, TextInput, Switch,
  Label, Spinner,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import {
  fetchDomainRateLimits, createDomainRateLimit,
  updateDomainRateLimit, deleteDomainRateLimit,
} from '../api/resources';
import type { DomainRateLimit } from '../types/api';

export default function DomainRateLimitPage(): React.ReactElement {
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [editItem, setEditItem] = useState<DomainRateLimit | null>(null);

  // Create form state
  const [domain, setDomain] = useState('');
  const [maxPerMinute, setMaxPerMinute] = useState('0');
  const [maxPerHour, setMaxPerHour] = useState('100');
  const [enabled, setEnabled] = useState(true);

  const { data: limits, isLoading } = useQuery({
    queryKey: ['domain-rate-limits'],
    queryFn: fetchDomainRateLimits,
  });

  const createMutation = useMutation({
    mutationFn: () => createDomainRateLimit({
      domain,
      max_per_minute: parseInt(maxPerMinute, 10) || 0,
      max_per_hour: parseInt(maxPerHour, 10) || 0,
      enabled,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domain-rate-limits'] });
      closeCreateModal();
    },
  });

  const updateMutation = useMutation({
    mutationFn: () => updateDomainRateLimit(editItem!.id, {
      max_per_minute: parseInt(maxPerMinute, 10) || 0,
      max_per_hour: parseInt(maxPerHour, 10) || 0,
      enabled,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domain-rate-limits'] });
      setEditItem(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteDomainRateLimit(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domain-rate-limits'] });
    },
  });

  const closeCreateModal = (): void => {
    setIsCreateOpen(false);
    setDomain('');
    setMaxPerMinute('0');
    setMaxPerHour('100');
    setEnabled(true);
  };

  const openEditModal = (item: DomainRateLimit): void => {
    setEditItem(item);
    setMaxPerMinute(String(item.max_per_minute));
    setMaxPerHour(String(item.max_per_hour));
    setEnabled(item.enabled);
  };

  const closeEditModal = (): void => {
    setEditItem(null);
  };

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div className="page-header">
        <Title headingLevel="h1" size="lg" className="page-title">
          Domain Rate Limits
        </Title>
        <Button onClick={() => setIsCreateOpen(true)}>Add Domain Limit</Button>
      </div>

      <Card>
        <CardBody>
          <p style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)', marginBottom: '1rem' }}>
            Limit the number of emails sent to specific destination domains per time window to prevent spam classification.
            A value of 0 means unlimited.
          </p>
          <Table aria-label="Domain rate limits">
            <Thead>
              <Tr>
                <Th>Domain</Th>
                <Th>Max / Minute</Th>
                <Th>Max / Hour</Th>
                <Th>Status</Th>
                <Th>Updated</Th>
                <Th>Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {limits?.map((limit) => (
                <Tr key={limit.id}>
                  <Td><strong>{limit.domain}</strong></Td>
                  <Td>{limit.max_per_minute || 'Unlimited'}</Td>
                  <Td>{limit.max_per_hour || 'Unlimited'}</Td>
                  <Td>
                    <Label color={limit.enabled ? 'green' : 'grey'}>
                      {limit.enabled ? 'Enabled' : 'Disabled'}
                    </Label>
                  </Td>
                  <Td>{new Date(limit.updated_at).toLocaleDateString()}</Td>
                  <Td>
                    <div className="action-buttons">
                      <Button variant="secondary" size="sm" onClick={() => openEditModal(limit)}>
                        Edit
                      </Button>
                      <Button variant="danger" size="sm"
                        onClick={() => { if (confirm(`Remove rate limit for ${limit.domain}?`)) deleteMutation.mutate(limit.id); }}
                        isDisabled={deleteMutation.isPending}>
                        Delete
                      </Button>
                    </div>
                  </Td>
                </Tr>
              ))}
              {(!limits || limits.length === 0) && (
                <Tr><Td colSpan={6}>No domain rate limits configured. All destinations are unlimited.</Td></Tr>
              )}
            </Tbody>
          </Table>
        </CardBody>
      </Card>

      {/* Create Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Add Domain Rate Limit"
        isOpen={isCreateOpen}
        onClose={closeCreateModal}
        actions={[
          <Button key="create" onClick={() => createMutation.mutate()}
            isDisabled={!domain || createMutation.isPending}>
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </Button>,
          <Button key="cancel" variant="link" onClick={closeCreateModal}>Cancel</Button>,
        ]}
      >
        <Form>
          {createMutation.isError && (
            <p className="feedback-message feedback-message--error">
              Failed to create domain rate limit.
            </p>
          )}
          <FormGroup label="Domain" isRequired fieldId="create-domain"
>
            <TextInput id="create-domain" value={domain}
              onChange={(_e, v) => setDomain(v.toLowerCase().trim())} isRequired />
          </FormGroup>
          <FormGroup label="Max per Minute" fieldId="create-max-min"
>
            <TextInput id="create-max-min" type="number" value={maxPerMinute}
              onChange={(_e, v) => setMaxPerMinute(v)} />
          </FormGroup>
          <FormGroup label="Max per Hour" fieldId="create-max-hour"
>
            <TextInput id="create-max-hour" type="number" value={maxPerHour}
              onChange={(_e, v) => setMaxPerHour(v)} />
          </FormGroup>
          <FormGroup fieldId="create-enabled">
            <Switch id="create-enabled" label="Enabled" labelOff="Disabled"
              isChecked={enabled} onChange={(_e, v) => setEnabled(v)} />
          </FormGroup>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        variant={ModalVariant.small}
        title={`Edit Rate Limit: ${editItem?.domain || ''}`}
        isOpen={!!editItem}
        onClose={closeEditModal}
        actions={[
          <Button key="save" onClick={() => updateMutation.mutate()}
            isDisabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Saving...' : 'Save'}
          </Button>,
          <Button key="cancel" variant="link" onClick={closeEditModal}>Cancel</Button>,
        ]}
      >
        <Form>
          {updateMutation.isError && (
            <p className="feedback-message feedback-message--error">
              Failed to update domain rate limit.
            </p>
          )}
          <FormGroup label="Max per Minute" fieldId="edit-max-min"
>
            <TextInput id="edit-max-min" type="number" value={maxPerMinute}
              onChange={(_e, v) => setMaxPerMinute(v)} />
          </FormGroup>
          <FormGroup label="Max per Hour" fieldId="edit-max-hour"
>
            <TextInput id="edit-max-hour" type="number" value={maxPerHour}
              onChange={(_e, v) => setMaxPerHour(v)} />
          </FormGroup>
          <FormGroup fieldId="edit-enabled">
            <Switch id="edit-enabled" label="Enabled" labelOff="Disabled"
              isChecked={enabled} onChange={(_e, v) => setEnabled(v)} />
          </FormGroup>
        </Form>
      </Modal>
    </PageSection>
  );
}
