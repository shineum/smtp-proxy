import { useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  PageSection, Title, Card, CardBody, CardTitle, DescriptionList, DescriptionListGroup,
  DescriptionListTerm, DescriptionListDescription, Label, Spinner,
  Button, Modal, ModalVariant,
  Form, FormGroup, TextInput, FormSelect, FormSelectOption,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchUser, resetUserPassword, fetchUserMemberships, fetchGroups, addGroupMember, removeMember, updateMemberRole, updatePasswordDisabled } from '../api/resources';
import { useAuth } from '../context/AuthContext';

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const { isSystemAdmin } = useAuth();
  const [isResetOpen, setIsResetOpen] = useState(false);
  const [newPassword, setNewPassword] = useState('');
  const [isAddGroupOpen, setIsAddGroupOpen] = useState(false);
  const [addGroupForm, setAddGroupForm] = useState({ group_id: '', role: 'member' });
  const [editingMembership, setEditingMembership] = useState<{ groupId: string; role: string } | null>(null);

  const { data: user, isLoading } = useQuery({
    queryKey: ['user', id],
    queryFn: () => fetchUser(id!),
    enabled: !!id,
  });

  const { data: memberships, isLoading: membershipsLoading } = useQuery({
    queryKey: ['user-memberships', id],
    queryFn: () => fetchUserMemberships(id!),
    enabled: !!id,
  });

  const { data: groups } = useQuery({
    queryKey: ['groups'],
    queryFn: fetchGroups,
    enabled: isAddGroupOpen,
  });

  const resetMutation = useMutation({
    mutationFn: () => resetUserPassword(id!, newPassword),
    onSuccess: () => {
      setIsResetOpen(false);
      setNewPassword('');
    },
  });

  const togglePasswordDisabledMutation = useMutation({
    mutationFn: (disabled: boolean) => updatePasswordDisabled(id!, disabled),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['user', id] }),
  });

  const addToGroupMutation = useMutation({
    mutationFn: () => addGroupMember(addGroupForm.group_id, id!, addGroupForm.role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-memberships', id] });
      setIsAddGroupOpen(false);
      setAddGroupForm({ group_id: '', role: 'member' });
    },
  });

  const removeFromGroupMutation = useMutation({
    mutationFn: (groupId: string) => removeMember(groupId, id!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['user-memberships', id] }),
  });

  const updateRoleMutation = useMutation({
    mutationFn: ({ groupId, role }: { groupId: string; role: string }) => updateMemberRole(groupId, id!, role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-memberships', id] });
      setEditingMembership(null);
    },
  });

  if (isLoading || !user) return <PageSection><Spinner size="xl" /></PageSection>;

  const memberGroupIds = new Set(memberships?.map(m => m.group_id) || []);
  const availableGroups = groups?.filter(g => !memberGroupIds.has(g.id)) || [];

  return (
    <PageSection>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <Title headingLevel="h1" size="lg">User: {user.email}</Title>
        {user.account_type === 'user' && (
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <Button
              variant={user.password_disabled ? 'primary' : 'warning'}
              onClick={() => togglePasswordDisabledMutation.mutate(!user.password_disabled)}
              isDisabled={togglePasswordDisabledMutation.isPending}
            >
              {user.password_disabled ? 'Enable Password' : 'Disable Password'}
            </Button>
            {!user.password_disabled && (
              <Button variant="secondary" onClick={() => setIsResetOpen(true)}>Reset Password</Button>
            )}
          </div>
        )}
      </div>

      <Card style={{ marginBottom: '1rem' }}>
        <CardBody>
          <DescriptionList>
            <DescriptionListGroup>
              <DescriptionListTerm>ID</DescriptionListTerm>
              <DescriptionListDescription>{user.id}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Email</DescriptionListTerm>
              <DescriptionListDescription>{user.email}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Username</DescriptionListTerm>
              <DescriptionListDescription>{user.username || '-'}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Account Type</DescriptionListTerm>
              <DescriptionListDescription>
                <Label color={user.account_type === 'smtp' ? 'orange' : 'blue'}>{user.account_type}</Label>
              </DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Status</DescriptionListTerm>
              <DescriptionListDescription>
                <Label color={user.status === 'active' ? 'green' : 'red'}>{user.status}</Label>
              </DescriptionListDescription>
            </DescriptionListGroup>
            {user.account_type === 'user' && (
              <DescriptionListGroup>
                <DescriptionListTerm>Password</DescriptionListTerm>
                <DescriptionListDescription>
                  <Label color={user.password_disabled ? 'orange' : 'green'}>
                    {user.password_disabled ? 'Disabled (SSO-only)' : 'Enabled'}
                  </Label>
                </DescriptionListDescription>
              </DescriptionListGroup>
            )}
            {user.allowed_domains && user.allowed_domains.length > 0 && (
              <DescriptionListGroup>
                <DescriptionListTerm>Allowed Domains</DescriptionListTerm>
                <DescriptionListDescription>{user.allowed_domains.join(', ')}</DescriptionListDescription>
              </DescriptionListGroup>
            )}
            {user.api_key && (
              <DescriptionListGroup>
                <DescriptionListTerm>API Key</DescriptionListTerm>
                <DescriptionListDescription><code>{user.api_key}</code></DescriptionListDescription>
              </DescriptionListGroup>
            )}
            <DescriptionListGroup>
              <DescriptionListTerm>Last Login</DescriptionListTerm>
              <DescriptionListDescription>
                {user.last_login ? new Date(user.last_login).toLocaleString() : 'Never'}
              </DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Created</DescriptionListTerm>
              <DescriptionListDescription>{new Date(user.created_at).toLocaleString()}</DescriptionListDescription>
            </DescriptionListGroup>
          </DescriptionList>
        </CardBody>
      </Card>

      <Card>
        <CardTitle>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>Group Memberships</span>
            {isSystemAdmin && (
              <Button variant="secondary" size="sm" onClick={() => setIsAddGroupOpen(true)}>Add to Group</Button>
            )}
          </div>
        </CardTitle>
        <CardBody>
          {membershipsLoading ? (
            <Spinner size="md" />
          ) : !memberships?.length ? (
            <p>No group memberships.</p>
          ) : (
            <Table aria-label="Memberships table" variant="compact">
              <Thead>
                <Tr>
                  <Th>Group</Th>
                  <Th>Type</Th>
                  <Th>Role</Th>
                  {isSystemAdmin && <Th>Actions</Th>}
                </Tr>
              </Thead>
              <Tbody>
                {memberships.map((m) => (
                  <Tr key={m.group_id}>
                    <Td>{m.group_name}</Td>
                    <Td><Label color={m.group_type === 'system' ? 'purple' : 'blue'}>{m.group_type}</Label></Td>
                    <Td>
                      {editingMembership?.groupId === m.group_id ? (
                        <span style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                          <FormSelect
                            value={editingMembership!.role}
                            onChange={(_e, v) => setEditingMembership(prev => prev ? { ...prev, role: v } : prev)}
                            aria-label="Role"
                            style={{ width: '120px' }}
                          >
                            <FormSelectOption value="member" label="Member" />
                            <FormSelectOption value="admin" label="Admin" />
                            <FormSelectOption value="owner" label="Owner" />
                          </FormSelect>
                          <Button
                            variant="primary"
                            size="sm"
                            isDisabled={updateRoleMutation.isPending}
                            onClick={() => editingMembership && updateRoleMutation.mutate({ groupId: editingMembership.groupId, role: editingMembership.role })}
                          >
                            Save
                          </Button>
                          <Button variant="link" size="sm" onClick={() => setEditingMembership(null)}>Cancel</Button>
                        </span>
                      ) : (
                        <Label>{m.role}</Label>
                      )}
                    </Td>
                    {isSystemAdmin && (
                      <Td>
                        {editingMembership?.groupId !== m.group_id && (
                          <>
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => setEditingMembership({ groupId: m.group_id, role: m.role })}
                              style={{ marginRight: '0.5rem' }}
                            >
                              Change Role
                            </Button>
                            <Button
                              variant="danger"
                              size="sm"
                              onClick={() => { if (confirm(`Remove from "${m.group_name}"?`)) removeFromGroupMutation.mutate(m.group_id); }}
                            >
                              Remove
                            </Button>
                          </>
                        )}
                      </Td>
                    )}
                  </Tr>
                ))}
              </Tbody>
            </Table>
          )}
        </CardBody>
      </Card>

      {/* Reset Password Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Reset Password"
        isOpen={isResetOpen}
        onClose={() => setIsResetOpen(false)}
        actions={[
          <Button key="reset" onClick={() => resetMutation.mutate()} isDisabled={!newPassword || resetMutation.isPending}>
            {resetMutation.isPending ? 'Resetting...' : 'Reset'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsResetOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="New Password" isRequired fieldId="new-password">
            <TextInput id="new-password" type="password" value={newPassword} onChange={(_e, v) => setNewPassword(v)} isRequired />
          </FormGroup>
        </Form>
      </Modal>

      {/* Add to Group Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Add to Group"
        isOpen={isAddGroupOpen}
        onClose={() => setIsAddGroupOpen(false)}
        actions={[
          <Button key="add" onClick={() => addToGroupMutation.mutate()} isDisabled={!addGroupForm.group_id || addToGroupMutation.isPending}>
            {addToGroupMutation.isPending ? 'Adding...' : 'Add'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsAddGroupOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="Group" isRequired fieldId="add-group-select">
            <FormSelect id="add-group-select" value={addGroupForm.group_id} onChange={(_e, v) => setAddGroupForm({ ...addGroupForm, group_id: v })}>
              <FormSelectOption value="" label="Select a group..." isPlaceholder />
              {availableGroups.map((g) => (
                <FormSelectOption key={g.id} value={g.id} label={g.name} />
              ))}
            </FormSelect>
          </FormGroup>
          <FormGroup label="Role" fieldId="add-group-role">
            <FormSelect id="add-group-role" value={addGroupForm.role} onChange={(_e, v) => setAddGroupForm({ ...addGroupForm, role: v })}>
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
