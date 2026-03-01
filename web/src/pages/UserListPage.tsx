import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Button, Modal, ModalVariant,
  Form, FormGroup, TextInput, FormSelect, FormSelectOption, Spinner, Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchUsers, createUser, deleteUser, updateUserStatus, fetchGroups } from '../api/resources';
import { useAuth } from '../context/AuthContext';

export default function UserListPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { me, isSystemAdmin } = useAuth();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [form, setForm] = useState({ email: '', password: '', account_type: 'user', username: '', role: 'member', group_id: '' });

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: fetchUsers,
  });

  const { data: groups } = useQuery({
    queryKey: ['groups'],
    queryFn: fetchGroups,
    enabled: isCreateOpen,
  });

  const currentGroupId = me?.current_group.group_id || '';

  const createMutation = useMutation({
    mutationFn: () => createUser({
      ...form,
      group_id: isSystemAdmin ? (form.group_id || currentGroupId) : currentGroupId,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setIsCreateOpen(false);
      setForm({ email: '', password: '', account_type: 'user', username: '', role: 'member', group_id: '' });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  });

  const toggleStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) => updateUserStatus(id, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  });

  const isSmtp = form.account_type === 'smtp';
  const canCreate = isSmtp ? !!form.username : !!form.email;

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <Title headingLevel="h1" size="lg">Users</Title>
        <Button onClick={() => setIsCreateOpen(true)}>Create User</Button>
      </div>

      <Table aria-label="Users table">
        <Thead>
          <Tr>
            <Th>Email</Th>
            <Th>Username</Th>
            <Th>Type</Th>
            <Th>Status</Th>
            <Th>Last Login</Th>
            <Th>Actions</Th>
          </Tr>
        </Thead>
        <Tbody>
          {users?.map((u) => (
            <Tr key={u.id} isClickable onRowClick={() => navigate(`/users/${u.id}`)}>
              <Td>{u.email}</Td>
              <Td>{u.username || '-'}</Td>
              <Td><Label color={u.account_type === 'smtp' ? 'orange' : 'blue'}>{u.account_type}</Label></Td>
              <Td><Label color={u.status === 'active' ? 'green' : 'red'}>{u.status}</Label></Td>
              <Td>{u.last_login ? new Date(u.last_login).toLocaleString() : 'Never'}</Td>
              <Td>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={(e) => { e.stopPropagation(); toggleStatusMutation.mutate({ id: u.id, status: u.status === 'active' ? 'suspended' : 'active' }); }}
                  style={{ marginRight: '0.5rem' }}
                >
                  {u.status === 'active' ? 'Suspend' : 'Activate'}
                </Button>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={(e) => { e.stopPropagation(); if (confirm(`Delete user "${u.email}"?`)) deleteMutation.mutate(u.id); }}
                >
                  Delete
                </Button>
              </Td>
            </Tr>
          ))}
        </Tbody>
      </Table>

      <Modal
        variant={ModalVariant.medium}
        title="Create User"
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        actions={[
          <Button key="create" onClick={() => createMutation.mutate()} isDisabled={!canCreate || createMutation.isPending}>
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsCreateOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="Account Type" fieldId="user-type">
            <FormSelect id="user-type" value={form.account_type} onChange={(_e, v) => setForm({ ...form, account_type: v, email: '', username: '' })}>
              <FormSelectOption value="user" label="Human User" />
              <FormSelectOption value="smtp" label="SMTP Account" />
            </FormSelect>
          </FormGroup>
          {isSmtp ? (
            <>
              <FormGroup label="Username" isRequired fieldId="user-username">
                <TextInput id="user-username" value={form.username} onChange={(_e, v) => setForm({ ...form, username: v })} isRequired />
              </FormGroup>
              <FormGroup label="Email (optional)" fieldId="user-email">
                <TextInput id="user-email" value={form.email} onChange={(_e, v) => setForm({ ...form, email: v })} placeholder="defaults to username@smtp.internal" />
              </FormGroup>
            </>
          ) : (
            <>
              <FormGroup label="Email" isRequired fieldId="user-email">
                <TextInput id="user-email" value={form.email} onChange={(_e, v) => setForm({ ...form, email: v })} isRequired />
              </FormGroup>
              <FormGroup label="Password" isRequired fieldId="user-password">
                <TextInput id="user-password" type="password" value={form.password} onChange={(_e, v) => setForm({ ...form, password: v })} isRequired />
              </FormGroup>
              <FormGroup label="Username" fieldId="user-username">
                <TextInput id="user-username" value={form.username} onChange={(_e, v) => setForm({ ...form, username: v })} />
              </FormGroup>
            </>
          )}
          <FormGroup label="Group" fieldId="user-group">
            {isSystemAdmin ? (
              <FormSelect id="user-group" value={form.group_id || currentGroupId} onChange={(_e, v) => setForm({ ...form, group_id: v })}>
                {groups?.map((g) => (
                  <FormSelectOption key={g.id} value={g.id} label={g.name} />
                ))}
              </FormSelect>
            ) : (
              <TextInput id="user-group" value={me?.memberships?.find(m => m.group_id === currentGroupId)?.group_name || 'Current Group'} isDisabled />
            )}
          </FormGroup>
          <FormGroup label="Role" fieldId="user-role">
            <FormSelect id="user-role" value={form.role} onChange={(_e, v) => setForm({ ...form, role: v })}>
              <FormSelectOption value="member" label="Member" />
              <FormSelectOption value="admin" label="Admin" />
              <FormSelectOption value="owner" label="Owner" />
            </FormSelect>
          </FormGroup>
        </Form>
      </Modal>
    </PageSection>
  );
}
