import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Button, Modal, ModalVariant,
  Form, FormGroup, TextInput, FormSelect, FormSelectOption, Spinner, Label,
  Checkbox, ClipboardCopy, Alert,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchUsers, createUser, deleteUser, updateUserStatus, fetchGroups, fetchProviders, fetchDeletedUsers, restoreUser, resetApiKey } from '../api/resources';
import { useAuth } from '../context/AuthContext';
import type { User } from '../types/api';

export default function UserListPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { me, isSystemAdmin } = useAuth();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [showDeleted, setShowDeleted] = useState(false);
  const [form, setForm] = useState({ email: '', password: '', account_type: 'user', username: '', role: 'member', group_id: '', password_disabled: true, provider_id: '' });
  const [resetKeyResult, setResetKeyResult] = useState<User | null>(null);

  const { data: users, isLoading } = useQuery({
    queryKey: showDeleted ? ['users-deleted'] : ['users'],
    queryFn: showDeleted ? fetchDeletedUsers : fetchUsers,
  });

  const { data: groups } = useQuery({
    queryKey: ['groups'],
    queryFn: fetchGroups,
    enabled: isCreateOpen,
  });

  const { data: providers } = useQuery({
    queryKey: ['providers'],
    queryFn: () => fetchProviders(),
    enabled: isCreateOpen && form.account_type === 'smtp',
  });

  const currentGroupId = me?.current_group.group_id || '';

  // Auto-select stdout provider for smtp accounts when providers load
  useEffect(() => {
    if (providers && form.account_type === 'smtp' && !form.provider_id) {
      const stdout = providers.find(p => p.provider_type === 'stdout' && p.enabled);
      if (stdout) setForm(f => ({ ...f, provider_id: stdout.id }));
    }
  }, [providers, form.account_type]);

  const createMutation = useMutation({
    mutationFn: () => {
      const payload: Record<string, unknown> = { ...form };
      if (!isSystemAdmin) {
        payload.group_id = currentGroupId;
      } else if (!form.group_id) {
        delete payload.group_id;
      }
      return createUser(payload);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setIsCreateOpen(false);
      setForm({ email: '', password: '', account_type: 'user', username: '', role: 'member', group_id: '', password_disabled: true, provider_id: '' });
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

  const restoreMutation = useMutation({
    mutationFn: (id: string) => restoreUser(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users-deleted'] });
      queryClient.invalidateQueries({ queryKey: ['users'] });
    },
  });

  const resetKeyMutation = useMutation({
    mutationFn: (id: string) => resetApiKey(id),
    onSuccess: (data) => {
      setResetKeyResult(data);
    },
  });

  const isSmtp = form.account_type === 'smtp';
  const canCreate = isSmtp ? !!form.username : (!!form.email && (form.password_disabled || !!form.password));

  if (isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div className="page-header">
        <Title headingLevel="h1" size="lg">Users</Title>
        <div className="action-buttons">
          <Button
            variant={showDeleted ? 'primary' : 'secondary'}
            onClick={() => setShowDeleted(!showDeleted)}
          >
            {showDeleted ? 'Show Active' : 'Show Deleted'}
          </Button>
          {!showDeleted && (
            <Button onClick={() => setIsCreateOpen(true)}>Create User</Button>
          )}
        </div>
      </div>

      {showDeleted && (
        <Alert
          variant="warning"
          isInline
          title="Soft-deleted users are permanently removed after 30 days"
          style={{ marginBottom: '1rem' }}
        />
      )}

      <Table aria-label="Users table">
        <Thead>
          <Tr>
            <Th>Email</Th>
            <Th>Username</Th>
            <Th>Type</Th>
            <Th>Status</Th>
            {showDeleted ? <Th>Deleted At</Th> : <Th>Last Login</Th>}
            <Th>Actions</Th>
          </Tr>
        </Thead>
        <Tbody>
          {users?.map((u) => (
            <Tr
              key={u.id}
              className={showDeleted ? 'deleted-row' : undefined}
              isClickable={!showDeleted}
              onRowClick={!showDeleted ? () => navigate(`/users/${u.id}`) : undefined}
            >
              <Td>{u.email}</Td>
              <Td>{u.username || '-'}</Td>
              <Td><Label color={u.account_type === 'smtp' ? 'orange' : 'blue'}>{u.account_type}</Label></Td>
              <Td><Label color={u.status === 'active' ? 'green' : 'red'}>{u.status}</Label></Td>
              {showDeleted ? (
                <Td>{u.deleted_at ? new Date(u.deleted_at).toLocaleString() : '-'}</Td>
              ) : (
                <Td>{u.last_login ? new Date(u.last_login).toLocaleString() : 'Never'}</Td>
              )}
              <Td>
                {showDeleted ? (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={(e) => { e.stopPropagation(); restoreMutation.mutate(u.id); }}
                    isDisabled={restoreMutation.isPending}
                  >
                    Restore
                  </Button>
                ) : (
                  <div className="action-buttons">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={(e) => { e.stopPropagation(); toggleStatusMutation.mutate({ id: u.id, status: u.status === 'active' ? 'suspended' : 'active' }); }}
                    >
                      {u.status === 'active' ? 'Suspend' : 'Activate'}
                    </Button>
                    {u.account_type === 'smtp' && (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation();
                          if (confirm('Reset API key? The current key will be invalidated immediately.')) {
                            resetKeyMutation.mutate(u.id);
                          }
                        }}
                        isDisabled={resetKeyMutation.isPending}
                      >
                        Reset Key
                      </Button>
                    )}
                    <Button
                      variant="danger"
                      size="sm"
                      onClick={(e) => { e.stopPropagation(); if (confirm(`Delete user "${u.email}"?`)) deleteMutation.mutate(u.id); }}
                    >
                      Delete
                    </Button>
                  </div>
                )}
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
            <FormSelect id="user-type" value={form.account_type} onChange={(_e, v) => setForm({ ...form, account_type: v, email: '', username: '', provider_id: '' })}>
              <FormSelectOption value="user" label="Team Member" />
              <FormSelectOption value="smtp" label="SMTP Account" />
            </FormSelect>
          </FormGroup>
          {isSmtp ? (
            <>
              <FormGroup label="Username" isRequired fieldId="user-username">
                <TextInput id="user-username" value={form.username} onChange={(_e, v) => setForm({ ...form, username: v })} isRequired />
              </FormGroup>
              <FormGroup label="Provider (defaults to stdout)" fieldId="user-provider">
                <FormSelect id="user-provider" value={form.provider_id} onChange={(_e, v) => setForm({ ...form, provider_id: v })}>
                  <FormSelectOption value="" label="Select a provider" isPlaceholder />
                  {providers?.filter(p => p.enabled).map((p) => (
                    <FormSelectOption key={p.id} value={p.id} label={`${p.name} (${p.provider_type})`} />
                  ))}
                </FormSelect>
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
              <FormGroup fieldId="user-password-disabled">
                <Checkbox
                  id="user-password-disabled"
                  label="Disable password (SSO-only account)"
                  isChecked={form.password_disabled}
                  onChange={(_e, checked) => setForm({ ...form, password_disabled: checked, password: '' })}
                />
              </FormGroup>
              {!form.password_disabled && (
                <FormGroup label="Password" isRequired fieldId="user-password">
                  <TextInput id="user-password" type="password" value={form.password} onChange={(_e, v) => setForm({ ...form, password: v })} isRequired />
                </FormGroup>
              )}
              <FormGroup label="Username" fieldId="user-username">
                <TextInput id="user-username" value={form.username} onChange={(_e, v) => setForm({ ...form, username: v })} />
              </FormGroup>
            </>
          )}
          <FormGroup label="Group" fieldId="user-group">
            {isSystemAdmin ? (
              <FormSelect id="user-group" value={form.group_id} onChange={(_e, v) => setForm({ ...form, group_id: v })}>
                <FormSelectOption value="" label="No group assignment" />
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

      {/* Reset API Key Result Modal */}
      <Modal
        variant={ModalVariant.small}
        title="API Key Reset"
        isOpen={!!resetKeyResult}
        onClose={() => setResetKeyResult(null)}
        actions={[
          <Button key="close" onClick={() => setResetKeyResult(null)}>Close</Button>,
        ]}
      >
        <div>
          <p style={{ marginBottom: '1rem' }}>API key has been reset. Copy the new key below - it will not be shown again.</p>
          <FormGroup label="New API Key" fieldId="reset-api-key">
            <ClipboardCopy isReadOnly className="mono">{resetKeyResult?.api_key || ''}</ClipboardCopy>
          </FormGroup>
        </div>
      </Modal>
    </PageSection>
  );
}
