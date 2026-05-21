import { useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  PageSection, Title, Card, CardBody, CardTitle, DescriptionList, DescriptionListGroup,
  DescriptionListTerm, DescriptionListDescription, Label, Spinner,
  Button, Modal, ModalVariant,
  Form, FormGroup, TextInput, FormSelect, FormSelectOption,
  Switch, TextArea, HelperText, HelperTextItem,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { fetchUser, resetUserPassword, fetchUserMemberships, fetchGroups, addGroupMember, removeMember, updateMemberRole, updatePasswordDisabled, updateUserAnonymous } from '../api/resources';
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
  const [isAnonOpen, setIsAnonOpen] = useState(false);
  const [anonAllowed, setAnonAllowed] = useState(false);
  const [anonCidrs, setAnonCidrs] = useState('');
  const [anonValidationError, setAnonValidationError] = useState('');

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

  const updateAnonMutation = useMutation({
    mutationFn: (data: { anonymous_allowed: boolean; anonymous_allowed_cidrs: string[] }) =>
      updateUserAnonymous(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user', id] });
      setIsAnonOpen(false);
    },
  });

  // Parse and validate CIDRs client-side (mirrors server rules)
  function parseCidrs(raw: string): { cidrs: string[]; error: string } {
    if (!raw.trim()) return { cidrs: [], error: '' };
    const lines = raw.split('\n').map(l => l.trim()).filter(Boolean);
    const seen = new Set<string>();
    const cidrRe = /^([0-9a-fA-F:.]+)\/(\d+)$/;
    for (const entry of lines) {
      if (!cidrRe.test(entry)) return { cidrs: [], error: `Invalid CIDR format: ${entry}` };
      const m = entry.match(cidrRe)!;
      const prefix = parseInt(m[2], 10);
      const isIPv6 = entry.includes(':');
      const maxPrefix = isIPv6 ? 128 : 32;
      if (prefix < 0 || prefix > maxPrefix) return { cidrs: [], error: `Prefix out of range: ${entry}` };
      if (entry === '0.0.0.0/0' || entry === '::/0') return { cidrs: [], error: `Catch-all prefix not allowed: ${entry}` };
      if (seen.has(entry)) return { cidrs: [], error: `Duplicate CIDR: ${entry}` };
      seen.add(entry);
    }
    if (lines.length > 64) return { cidrs: [], error: 'Maximum 64 CIDR entries allowed' };
    return { cidrs: lines, error: '' };
  }

  function openAnonModal() {
    setAnonAllowed(user?.anonymous_allowed ?? false);
    setAnonCidrs((user?.anonymous_allowed_cidrs ?? []).join('\n'));
    setAnonValidationError('');
    updateAnonMutation.reset();
    setIsAnonOpen(true);
  }

  function handleAnonSave() {
    const { cidrs, error } = parseCidrs(anonCidrs);
    if (anonAllowed && cidrs.length === 0) {
      setAnonValidationError(error || 'At least one CIDR is required when anonymous access is enabled.');
      return;
    }
    if (!anonAllowed && anonCidrs.trim()) {
      setAnonValidationError('Clear the CIDR list when disabling anonymous access.');
      return;
    }
    if (error) { setAnonValidationError(error); return; }
    setAnonValidationError('');
    updateAnonMutation.mutate({ anonymous_allowed: anonAllowed, anonymous_allowed_cidrs: cidrs });
  }

  if (isLoading || !user) return <PageSection><Spinner size="xl" /></PageSection>;

  const memberGroupIds = new Set(memberships?.map(m => m.group_id) || []);
  const availableGroups = groups?.filter(g => !memberGroupIds.has(g.id)) || [];

  return (
    <PageSection>
      <div className="page-header">
        <Title headingLevel="h1" size="lg">User: {user.email}</Title>
        {user.account_type === 'user' && (
          <div className="action-buttons">
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

      <Card className="card-spaced">
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
            {user.account_type === 'smtp' && (
              <>
                <DescriptionListGroup>
                  <DescriptionListTerm>Anonymous SMTP</DescriptionListTerm>
                  <DescriptionListDescription>
                    <span style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Label color={user.anonymous_allowed ? 'green' : 'grey'}>
                        {user.anonymous_allowed ? 'Allowed' : 'Disabled'}
                      </Label>
                      {isSystemAdmin && (
                        <Button variant="secondary" size="sm" onClick={openAnonModal}>
                          Edit anonymous access
                        </Button>
                      )}
                    </span>
                  </DescriptionListDescription>
                </DescriptionListGroup>
                {user.anonymous_allowed && user.anonymous_allowed_cidrs && user.anonymous_allowed_cidrs.length > 0 && (
                  <DescriptionListGroup>
                    <DescriptionListTerm>Allowed CIDRs</DescriptionListTerm>
                    <DescriptionListDescription>
                      <span style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                        {user.anonymous_allowed_cidrs.map(cidr => (
                          <Label key={cidr} color="cyan">{cidr}</Label>
                        ))}
                      </span>
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                )}
              </>
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
          <div className="card-section-header">
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
                        <span className="action-buttons">
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
                            <div className="action-buttons">
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => setEditingMembership({ groupId: m.group_id, role: m.role })}
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
                          </div>
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

      {/* Anonymous Access Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Edit Anonymous SMTP Access"
        isOpen={isAnonOpen}
        onClose={() => setIsAnonOpen(false)}
        actions={[
          <Button
            key="save"
            variant="primary"
            onClick={handleAnonSave}
            isDisabled={updateAnonMutation.isPending}
          >
            {updateAnonMutation.isPending ? 'Saving...' : 'Save'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsAnonOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup fieldId="anon-switch" label="Allow anonymous submissions">
            <Switch
              id="anon-switch"
              isChecked={anonAllowed}
              onChange={(_e, checked) => {
                setAnonAllowed(checked);
                if (!checked) setAnonCidrs('');
                setAnonValidationError('');
              }}
              label="Enabled"
              labelOff="Disabled"
            />
          </FormGroup>
          {anonAllowed && (
            <FormGroup
              label="Allowed CIDRs (one per line)"
              isRequired
              fieldId="anon-cidrs"
            >
              <TextArea
                id="anon-cidrs"
                value={anonCidrs}
                onChange={(_e, v) => { setAnonCidrs(v); setAnonValidationError(''); }}
                rows={6}
                placeholder={"10.1.1.0/24\n192.168.0.0/16"}
                validated={anonValidationError ? 'error' : 'default'}
              />
              <HelperText>
                <HelperTextItem>
                  Enter IPv4 or IPv6 CIDR prefixes, one per line. Max 64 entries. Catch-all (0.0.0.0/0, ::/0) not permitted.
                </HelperTextItem>
              </HelperText>
            </FormGroup>
          )}
          {(anonValidationError || updateAnonMutation.isError) && (
            <HelperText>
              <HelperTextItem variant="error">
                {anonValidationError ||
                  (() => {
                    const err = updateAnonMutation.error as { response?: { data?: { error?: string } } };
                    return err?.response?.data?.error ?? 'An error occurred. Please try again.';
                  })()}
              </HelperTextItem>
            </HelperText>
          )}
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
