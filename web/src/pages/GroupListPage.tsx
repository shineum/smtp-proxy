import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Button, Modal, ModalVariant,
  Form, FormGroup, TextInput, Spinner,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchGroups, createGroup, deleteGroup } from '../api/resources';
import { useAuth } from '../context/AuthContext';

export default function GroupListPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { refreshProfile } = useAuth();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newName, setNewName] = useState('');
  const [newLimit, setNewLimit] = useState('');

  const { data: groups, isLoading } = useQuery({
    queryKey: ['groups'],
    queryFn: fetchGroups,
  });

  const createMutation = useMutation({
    mutationFn: () => createGroup(newName, newLimit ? parseInt(newLimit) : undefined),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      refreshProfile();
      setIsCreateOpen(false);
      setNewName('');
      setNewLimit('');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteGroup,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['groups'] }),
  });

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <Title headingLevel="h1" size="lg">Groups</Title>
        <Button onClick={() => setIsCreateOpen(true)}>Create Group</Button>
      </div>

      <Table aria-label="Groups table">
        <Thead>
          <Tr>
            <Th>Name</Th>
            <Th>Type</Th>
            <Th>Status</Th>
            <Th>Monthly Sent / Limit</Th>
            <Th>Created</Th>
            <Th>Actions</Th>
          </Tr>
        </Thead>
        <Tbody>
          {groups?.map((g) => (
            <Tr key={g.id} isClickable onRowClick={() => navigate(`/groups/${g.id}`)}>
              <Td>{g.name}</Td>
              <Td><Label color={g.group_type === 'system' ? 'purple' : 'blue'}>{g.group_type}</Label></Td>
              <Td><Label color={g.status === 'active' ? 'green' : 'red'}>{g.status}</Label></Td>
              <Td>{g.monthly_sent.toLocaleString()} / {g.monthly_limit === 0 ? 'unlimited' : g.monthly_limit.toLocaleString()}</Td>
              <Td>{new Date(g.created_at).toLocaleDateString()}</Td>
              <Td>
                {g.group_type !== 'system' && (
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={(e) => { e.stopPropagation(); if (confirm(`Delete group "${g.name}"?`)) deleteMutation.mutate(g.id); }}
                  >
                    Delete
                  </Button>
                )}
              </Td>
            </Tr>
          ))}
        </Tbody>
      </Table>

      <Modal
        variant={ModalVariant.small}
        title="Create Group"
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        actions={[
          <Button key="create" onClick={() => createMutation.mutate()} isDisabled={!newName || createMutation.isPending}>
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsCreateOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="Name" isRequired fieldId="group-name">
            <TextInput id="group-name" value={newName} onChange={(_e, v) => setNewName(v)} isRequired />
          </FormGroup>
          <FormGroup label="Monthly Limit (0 = unlimited)" fieldId="group-limit">
            <TextInput id="group-limit" type="number" value={newLimit} onChange={(_e, v) => setNewLimit(v)} />
          </FormGroup>
        </Form>
      </Modal>
    </PageSection>
  );
}
