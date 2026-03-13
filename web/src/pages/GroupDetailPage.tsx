import { useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Tabs, Tab, TabTitleText,
  Card, CardBody, DescriptionList, DescriptionListGroup,
  DescriptionListTerm, DescriptionListDescription, Label, Spinner,
  Button, Modal, ModalVariant, Form, FormGroup, TextInput,
  FormSelect, FormSelectOption, ClipboardCopy, Switch,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useState, useEffect, Fragment } from 'react';
import {
  fetchGroup, fetchGroupMembers, fetchActivityLogs,
  addGroupMember, removeMember, updateMemberRole,
  createServiceAccount, updateServiceAccount, fetchUser,
  fetchProviders, updateGroup,
  fetchApiKeys, createApiKey, updateApiKeyStatus, deleteApiKey,
} from '../api/resources';
import { useAuth } from '../context/AuthContext';
import type { ApiKeyInfo } from '../types/api';

export default function GroupDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { me, isSystemAdmin } = useAuth();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState(0);

  // Service account creation state
  const [isCreateSAOpen, setIsCreateSAOpen] = useState(false);
  const [saUsername, setSaUsername] = useState('');
  const [saEmail, setSaEmail] = useState('');
  const [saDomains, setSaDomains] = useState('');
  const [saProviderId, setSaProviderId] = useState('');

  // Edit service account state
  const [isEditSAOpen, setIsEditSAOpen] = useState(false);
  const [editSAUserId, setEditSAUserId] = useState('');
  const [editSADomains, setEditSADomains] = useState('');
  const [editSAProviderId, setEditSAProviderId] = useState('');

  // Add member state
  const [isAddMemberOpen, setIsAddMemberOpen] = useState(false);
  const [addMemberUserId, setAddMemberUserId] = useState('');
  const [addMemberRole, setAddMemberRole] = useState('member');

  // Edit group state
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [editName, setEditName] = useState('');
  const [editMonthlyLimit, setEditMonthlyLimit] = useState(0);
  const [editDisplayName, setEditDisplayName] = useState('');
  const [editDescription, setEditDescription] = useState('');

  // API key management state
  const [expandedSA, setExpandedSA] = useState<string | null>(null);
  const [isCreateKeyOpen, setIsCreateKeyOpen] = useState(false);
  const [createKeyUserId, setCreateKeyUserId] = useState('');
  const [createKeyLabel, setCreateKeyLabel] = useState('');
  const [createKeyExpiry, setCreateKeyExpiry] = useState('');
  const [createdKeyResult, setCreatedKeyResult] = useState<ApiKeyInfo | null>(null);

  const callerRole = me?.memberships?.find(m => m.group_id === id)?.role;
  const isOwnerOrAdmin = isSystemAdmin || callerRole === 'owner' || callerRole === 'admin';

  const { data: group, isLoading } = useQuery({
    queryKey: ['group', id],
    queryFn: () => fetchGroup(id!),
    enabled: !!id,
  });

  const { data: members } = useQuery({
    queryKey: ['group-members', id],
    queryFn: () => fetchGroupMembers(id!),
    enabled: !!id && activeTab === 1,
  });

  const { data: activity } = useQuery({
    queryKey: ['group-activity', id],
    queryFn: () => fetchActivityLogs(id!),
    enabled: !!id && activeTab === 2,
  });

  const { data: providers } = useQuery({
    queryKey: ['providers', id],
    queryFn: () => fetchProviders(id!),
    enabled: (isCreateSAOpen || isEditSAOpen) && !!id,
  });

  const { data: apiKeys, refetch: refetchApiKeys } = useQuery({
    queryKey: ['api-keys', id, expandedSA],
    queryFn: () => fetchApiKeys(id!, expandedSA!),
    enabled: !!id && !!expandedSA,
  });

  // Auto-select stdout provider as default when providers load
  useEffect(() => {
    if (providers && !saProviderId) {
      const stdout = providers.find(p => p.provider_type === 'stdout' && p.enabled);
      if (stdout) setSaProviderId(stdout.id);
    }
  }, [providers]);


  const createSAMutation = useMutation({
    mutationFn: () => {
      const domains = saDomains.trim() ? saDomains.split(',').map(d => d.trim()).filter(Boolean) : undefined;
      return createServiceAccount(id!, {
        username: saUsername,
        email: saEmail || undefined,
        allowed_domains: domains,
        provider_id: saProviderId || undefined,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group-members', id] });
      setIsCreateSAOpen(false);
      setSaUsername('');
      setSaEmail('');
      setSaDomains('');
      setSaProviderId('');
    },
  });

  const addMemberMutation = useMutation({
    mutationFn: () => addGroupMember(id!, addMemberUserId, addMemberRole),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group-members', id] });
      setIsAddMemberOpen(false);
      setAddMemberUserId('');
      setAddMemberRole('member');
    },
  });

  const removeMemberMutation = useMutation({
    mutationFn: (userId: string) => removeMember(id!, userId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['group-members', id] }),
  });

  const updateRoleMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: string }) => updateMemberRole(id!, userId, role),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['group-members', id] }),
  });

  const editGroupMutation = useMutation({
    mutationFn: () => updateGroup(id!, { name: editName, monthly_limit: editMonthlyLimit, display_name: editDisplayName || undefined, description: editDescription || undefined }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group', id] });
      setIsEditOpen(false);
    },
  });

  const editSAMutation = useMutation({
    mutationFn: () => {
      const domains = editSADomains.trim() ? editSADomains.split(',').map(d => d.trim()).filter(Boolean) : [];
      return updateServiceAccount(id!, editSAUserId, {
        allowed_domains: domains,
        provider_id: editSAProviderId || undefined,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group-members', id] });
      setIsEditSAOpen(false);
    },
  });

  const createKeyMutation = useMutation({
    mutationFn: () => createApiKey(id!, createKeyUserId, {
      label: createKeyLabel || 'default',
      api_key_expires_in: createKeyExpiry || undefined,
    }),
    onSuccess: (data) => {
      setCreatedKeyResult(data);
      refetchApiKeys();
      setCreateKeyLabel('');
      setCreateKeyExpiry('');
    },
  });

  const toggleKeyMutation = useMutation({
    mutationFn: ({ userId, keyId, isActive }: { userId: string; keyId: string; isActive: boolean }) =>
      updateApiKeyStatus(id!, userId, keyId, isActive),
    onSuccess: () => refetchApiKeys(),
  });

  const deleteKeyMutation = useMutation({
    mutationFn: ({ userId, keyId }: { userId: string; keyId: string }) =>
      deleteApiKey(id!, userId, keyId),
    onSuccess: () => refetchApiKeys(),
  });

  const openEditSAModal = async (userId: string) => {
    setEditSAUserId(userId);
    try {
      const user = await fetchUser(userId);
      setEditSADomains(user.allowed_domains?.join(', ') || '');
      setEditSAProviderId(user.provider_id || '');
    } catch {
      setEditSADomains('');
      setEditSAProviderId('');
    }
    setIsEditSAOpen(true);
  };

  const openEditModal = () => {
    setEditName(group?.name || '');
    setEditMonthlyLimit(group?.monthly_limit || 0);
    setEditDisplayName(group?.display_name || '');
    setEditDescription(group?.description || '');
    setIsEditOpen(true);
  };

  if (isLoading || !group) return <PageSection><Spinner size="xl" /></PageSection>;

  const serviceAccounts = members?.filter(m => m.email?.endsWith('@smtp.internal') || false) || [];
  const humanMembers = members?.filter(m => !m.email?.endsWith('@smtp.internal')) || [];

  return (
    <PageSection>
      <div className="page-header">
        <Title headingLevel="h1" size="lg">
          Group: {group.display_name || group.name}
        </Title>
        {isOwnerOrAdmin && (
          <Button variant="secondary" size="sm" onClick={openEditModal}>Edit</Button>
        )}
      </div>

      <Tabs activeKey={activeTab} onSelect={(_e, k) => setActiveTab(k as number)}>
        <Tab eventKey={0} title={<TabTitleText>Details</TabTitleText>}>
          <Card style={{ marginTop: '1rem' }}>
            <CardBody>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>ID</DescriptionListTerm>
                  <DescriptionListDescription className="mono">{group.id}</DescriptionListDescription>
                </DescriptionListGroup>
                {group.display_name && (
                  <DescriptionListGroup>
                    <DescriptionListTerm>Display Name</DescriptionListTerm>
                    <DescriptionListDescription>{group.display_name}</DescriptionListDescription>
                  </DescriptionListGroup>
                )}
                {group.description && (
                  <DescriptionListGroup>
                    <DescriptionListTerm>Description</DescriptionListTerm>
                    <DescriptionListDescription>{group.description}</DescriptionListDescription>
                  </DescriptionListGroup>
                )}
                <DescriptionListGroup>
                  <DescriptionListTerm>SMTP Auth Key</DescriptionListTerm>
                  <DescriptionListDescription>
                    <ClipboardCopy isReadOnly hoverTip="Copy" clickTip="Copied" className="mono">
                      {group.id}
                    </ClipboardCopy>
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Type</DescriptionListTerm>
                  <DescriptionListDescription><Label color={group.group_type === 'system' ? 'purple' : 'blue'}>{group.group_type}</Label></DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Status</DescriptionListTerm>
                  <DescriptionListDescription><Label color={group.status === 'active' ? 'green' : 'red'}>{group.status}</Label></DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Monthly Usage</DescriptionListTerm>
                  <DescriptionListDescription>{group.monthly_sent} / {group.monthly_limit === 0 ? 'unlimited' : group.monthly_limit}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Created</DescriptionListTerm>
                  <DescriptionListDescription>{new Date(group.created_at).toLocaleString()}</DescriptionListDescription>
                </DescriptionListGroup>
              </DescriptionList>
            </CardBody>
          </Card>
        </Tab>

        <Tab eventKey={1} title={<TabTitleText>Members</TabTitleText>}>
          <Card style={{ marginTop: '1rem' }}>
            <CardBody>
              {isOwnerOrAdmin && (
                <div style={{ marginBottom: '1rem' }}>
                  <Button onClick={() => setIsAddMemberOpen(true)}>Add Member</Button>
                </div>
              )}
              <Table aria-label="Members">
                <Thead><Tr><Th>Email</Th><Th>Username</Th><Th>Role</Th><Th>Joined</Th>{isOwnerOrAdmin && <Th>Actions</Th>}</Tr></Thead>
                <Tbody>
                  {humanMembers.map((m) => (
                    <Tr key={m.id}>
                      <Td>{m.email || m.user_id}</Td>
                      <Td>{m.username || '-'}</Td>
                      <Td><Label>{m.role}</Label></Td>
                      <Td>{new Date(m.created_at).toLocaleDateString()}</Td>
                      {isOwnerOrAdmin && (
                        <Td>
                          <div className="action-buttons">
                            {m.role !== 'admin' && (
                              <Button variant="secondary" size="sm" onClick={() => updateRoleMutation.mutate({ userId: m.user_id, role: 'admin' })}>
                                Make Admin
                              </Button>
                            )}
                            {m.role !== 'member' && m.role !== 'owner' && (
                              <Button variant="secondary" size="sm" onClick={() => updateRoleMutation.mutate({ userId: m.user_id, role: 'member' })}>
                                Make Member
                              </Button>
                            )}
                            <Button variant="danger" size="sm" onClick={() => { if (confirm('Remove this member?')) removeMemberMutation.mutate(m.user_id); }}>
                              Remove
                            </Button>
                          </div>
                        </Td>
                      )}
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            </CardBody>
          </Card>

          <Card style={{ marginTop: '1rem' }}>
            <CardBody>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
                <Title headingLevel="h3" size="md">Service Accounts</Title>
                {isOwnerOrAdmin && (
                  <Button onClick={() => setIsCreateSAOpen(true)}>Create Service Account</Button>
                )}
              </div>
              <Table aria-label="Service accounts">
                <Thead><Tr><Th></Th><Th>Username</Th><Th>Email</Th><Th>Role</Th><Th>Joined</Th>{isOwnerOrAdmin && <Th>Actions</Th>}</Tr></Thead>
                <Tbody>
                  {serviceAccounts.map((m) => (
                    <Fragment key={m.id}>
                      <Tr>
                        <Td>
                          <Button variant="plain" size="sm"
                            onClick={() => setExpandedSA(expandedSA === m.user_id ? null : m.user_id)}>
                            {expandedSA === m.user_id ? '▼' : '▶'}
                          </Button>
                        </Td>
                        <Td>{m.username || '-'}</Td>
                        <Td>{m.email || m.user_id}</Td>
                        <Td><Label>{m.role}</Label></Td>
                        <Td>{new Date(m.created_at).toLocaleDateString()}</Td>
                        {isOwnerOrAdmin && (
                          <Td>
                            <div className="action-buttons">
                              <Button variant="secondary" size="sm" onClick={() => openEditSAModal(m.user_id)}>
                                Edit
                              </Button>
                              <Button variant="danger" size="sm" onClick={() => { if (confirm('Remove this service account?')) removeMemberMutation.mutate(m.user_id); }}>
                                Remove
                              </Button>
                            </div>
                          </Td>
                        )}
                      </Tr>
                      {expandedSA === m.user_id && (
                        <Tr key={`${m.id}-keys`}>
                          <Td colSpan={isOwnerOrAdmin ? 6 : 5}>
                            <div style={{ padding: '1rem', background: 'var(--pf-v5-global--BackgroundColor--200)' }}>
                              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
                                <Title headingLevel="h4" size="md">API Keys</Title>
                                {isOwnerOrAdmin && (
                                  <Button size="sm" onClick={() => {
                                    setCreateKeyUserId(m.user_id);
                                    setIsCreateKeyOpen(true);
                                    setCreatedKeyResult(null);
                                  }}>Add Key</Button>
                                )}
                              </div>
                              <Table aria-label="API keys" variant="compact">
                                <Thead><Tr><Th>Prefix</Th><Th>Label</Th><Th>Active</Th><Th>Expires</Th><Th>Last Used</Th><Th>Created</Th>{isOwnerOrAdmin && <Th>Actions</Th>}</Tr></Thead>
                                <Tbody>
                                  {apiKeys?.map((k) => (
                                    <Tr key={k.id}>
                                      <Td><code>{k.key_prefix}</code></Td>
                                      <Td>{k.label || '-'}</Td>
                                      <Td>
                                        {isOwnerOrAdmin ? (
                                          <Switch
                                            id={`key-active-${k.id}`}
                                            isChecked={k.is_active}
                                            onChange={(_e, checked) => toggleKeyMutation.mutate({ userId: m.user_id, keyId: k.id, isActive: checked })}
                                            isDisabled={toggleKeyMutation.isPending}
                                          />
                                        ) : (
                                          <Label color={k.is_active ? 'green' : 'grey'}>{k.is_active ? 'Active' : 'Inactive'}</Label>
                                        )}
                                      </Td>
                                      <Td>{k.expires_at ? new Date(k.expires_at).toLocaleDateString() : 'Never'}</Td>
                                      <Td>{k.last_used_at ? new Date(k.last_used_at).toLocaleString() : 'Never'}</Td>
                                      <Td>{new Date(k.created_at).toLocaleDateString()}</Td>
                                      {isOwnerOrAdmin && (
                                        <Td>
                                          <Button variant="danger" size="sm"
                                            onClick={() => { if (confirm('Delete this API key?')) deleteKeyMutation.mutate({ userId: m.user_id, keyId: k.id }); }}
                                            isDisabled={deleteKeyMutation.isPending}>
                                            Delete
                                          </Button>
                                        </Td>
                                      )}
                                    </Tr>
                                  ))}
                                  {(!apiKeys || apiKeys.length === 0) && (
                                    <Tr><Td colSpan={isOwnerOrAdmin ? 7 : 6}>No API keys. Add one to enable SMTP authentication.</Td></Tr>
                                  )}
                                </Tbody>
                              </Table>
                            </div>
                          </Td>
                        </Tr>
                      )}
                    </Fragment>
                  ))}
                  {serviceAccounts.length === 0 && (
                    <Tr><Td colSpan={isOwnerOrAdmin ? 6 : 5}>No service accounts</Td></Tr>
                  )}
                </Tbody>
              </Table>
            </CardBody>
          </Card>
        </Tab>

        <Tab eventKey={2} title={<TabTitleText>Activity</TabTitleText>}>
          <Card style={{ marginTop: '1rem' }}>
            <CardBody>
              <Table aria-label="Activity logs">
                <Thead><Tr><Th>Action</Th><Th>Resource</Th><Th>Time</Th></Tr></Thead>
                <Tbody>
                  {activity?.map((a) => (
                    <Tr key={a.id}>
                      <Td>{a.action}</Td>
                      <Td>{a.resource_type}{a.resource_id ? ` (${a.resource_id.slice(0, 8)}...)` : ''}</Td>
                      <Td>{new Date(a.created_at).toLocaleString()}</Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            </CardBody>
          </Card>
        </Tab>
      </Tabs>

      {/* Create Service Account Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Create Service Account"
        isOpen={isCreateSAOpen}
        onClose={() => setIsCreateSAOpen(false)}
        actions={[
          <Button key="create" onClick={() => createSAMutation.mutate()} isDisabled={!saUsername || createSAMutation.isPending}>
            {createSAMutation.isPending ? 'Creating...' : 'Create'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsCreateSAOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="Username" isRequired fieldId="sa-username">
            <TextInput id="sa-username" value={saUsername} onChange={(_e, v) => setSaUsername(v)} isRequired />
          </FormGroup>
          <FormGroup label="Provider (defaults to stdout)" fieldId="sa-provider">
            <FormSelect id="sa-provider" value={saProviderId} onChange={(_e, v) => setSaProviderId(v)}>
              <FormSelectOption value="" label="Select a provider" isPlaceholder />
              {providers?.filter(p => p.enabled).map((p) => (
                <FormSelectOption key={p.id} value={p.id} label={`${p.name} (${p.provider_type})`} />
              ))}
            </FormSelect>
          </FormGroup>
          <FormGroup label="Email (optional, defaults to username@smtp.internal)" fieldId="sa-email">
            <TextInput id="sa-email" value={saEmail} onChange={(_e, v) => setSaEmail(v)} />
          </FormGroup>
          <FormGroup label="Allowed Domains (comma-separated, optional)" fieldId="sa-domains">
            <TextInput id="sa-domains" value={saDomains} onChange={(_e, v) => setSaDomains(v)} placeholder="example.com, other.com" />
          </FormGroup>
          {createSAMutation.isError && (
            <p style={{ color: 'red' }}>Failed to create service account. Username may already be in use.</p>
          )}
        </Form>
      </Modal>

      {/* Add Member Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Add Member"
        isOpen={isAddMemberOpen}
        onClose={() => setIsAddMemberOpen(false)}
        actions={[
          <Button key="add" onClick={() => addMemberMutation.mutate()} isDisabled={!addMemberUserId || addMemberMutation.isPending}>
            {addMemberMutation.isPending ? 'Adding...' : 'Add'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsAddMemberOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="User ID" isRequired fieldId="member-user-id">
            <TextInput id="member-user-id" value={addMemberUserId} onChange={(_e, v) => setAddMemberUserId(v)} isRequired />
          </FormGroup>
          <FormGroup label="Role" fieldId="member-role">
            <TextInput id="member-role" value={addMemberRole} onChange={(_e, v) => setAddMemberRole(v)} placeholder="member" />
          </FormGroup>
          {addMemberMutation.isError && (
            <p style={{ color: 'red' }}>Failed to add member. User may not exist or is already a member.</p>
          )}
        </Form>
      </Modal>

      {/* Edit Group Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Edit Group"
        isOpen={isEditOpen}
        onClose={() => setIsEditOpen(false)}
        actions={[
          <Button key="save" onClick={() => editGroupMutation.mutate()} isDisabled={!editName || editGroupMutation.isPending}>
            {editGroupMutation.isPending ? 'Saving...' : 'Save'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsEditOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="Name" isRequired fieldId="edit-name">
            <TextInput id="edit-name" value={editName} onChange={(_e, v) => setEditName(v)} isRequired />
          </FormGroup>
          <FormGroup label="Display Name" fieldId="edit-display-name">
            <TextInput id="edit-display-name" value={editDisplayName} onChange={(_e, v) => setEditDisplayName(v)} />
          </FormGroup>
          <FormGroup label="Description" fieldId="edit-description">
            <TextInput id="edit-description" value={editDescription} onChange={(_e, v) => setEditDescription(v)} />
          </FormGroup>
          <FormGroup label="Monthly Limit (0 = unlimited)" fieldId="edit-monthly-limit">
            <TextInput id="edit-monthly-limit" type="number" value={String(editMonthlyLimit)} onChange={(_e, v) => setEditMonthlyLimit(Number(v) || 0)} />
          </FormGroup>
          {editGroupMutation.isError && (
            <p style={{ color: 'red' }}>Failed to update group.</p>
          )}
        </Form>
      </Modal>

      {/* Edit Service Account Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Edit Service Account"
        isOpen={isEditSAOpen}
        onClose={() => setIsEditSAOpen(false)}
        actions={[
          <Button key="save" onClick={() => editSAMutation.mutate()} isDisabled={editSAMutation.isPending}>
            {editSAMutation.isPending ? 'Saving...' : 'Save'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsEditSAOpen(false)}>Cancel</Button>,
        ]}
      >
        <Form>
          <FormGroup label="Allowed Domains (comma-separated)" fieldId="edit-sa-domains">
            <TextInput id="edit-sa-domains" value={editSADomains} onChange={(_e, v) => setEditSADomains(v)} placeholder="example.com, other.com" />
          </FormGroup>
          <FormGroup label="Provider" fieldId="edit-sa-provider">
            <FormSelect id="edit-sa-provider" value={editSAProviderId} onChange={(_e, v) => setEditSAProviderId(v)}>
              <FormSelectOption value="" label="Select a provider" isPlaceholder />
              {providers?.filter(p => p.enabled).map((p) => (
                <FormSelectOption key={p.id} value={p.id} label={`${p.name} (${p.provider_type})`} />
              ))}
            </FormSelect>
          </FormGroup>
          {editSAMutation.isError && (
            <p style={{ color: 'red' }}>Failed to update service account.</p>
          )}
        </Form>
      </Modal>

      {/* Create API Key Modal */}
      <Modal
        variant={ModalVariant.small}
        title="Create API Key"
        isOpen={isCreateKeyOpen}
        onClose={() => { setIsCreateKeyOpen(false); setCreatedKeyResult(null); }}
        actions={createdKeyResult ? [
          <Button key="close" onClick={() => { setIsCreateKeyOpen(false); setCreatedKeyResult(null); }}>Close</Button>,
        ] : [
          <Button key="create" onClick={() => createKeyMutation.mutate()} isDisabled={createKeyMutation.isPending}>
            {createKeyMutation.isPending ? 'Creating...' : 'Create Key'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsCreateKeyOpen(false)}>Cancel</Button>,
        ]}
      >
        {createdKeyResult ? (
          <div>
            <p style={{ marginBottom: '1rem' }}>API key created. Copy it now - it will not be shown again.</p>
            <FormGroup label="API Key" fieldId="new-api-key">
              <ClipboardCopy isReadOnly className="mono">{createdKeyResult.api_key || ''}</ClipboardCopy>
            </FormGroup>
            {createdKeyResult.expires_at && (
              <p style={{ marginTop: '0.5rem', color: 'var(--pf-v5-global--Color--200)' }}>
                Expires: {new Date(createdKeyResult.expires_at).toLocaleString()}
              </p>
            )}
          </div>
        ) : (
          <Form>
            <FormGroup label="Label" fieldId="key-label">
              <TextInput id="key-label" value={createKeyLabel} onChange={(_e, v) => setCreateKeyLabel(v)} placeholder="e.g. production, ci-pipeline" />
            </FormGroup>
            <FormGroup label="Expiration" fieldId="key-expiry">
              <FormSelect id="key-expiry" value={createKeyExpiry} onChange={(_e, v) => setCreateKeyExpiry(v)}>
                <FormSelectOption value="" label="No expiration" />
                <FormSelectOption value="1d" label="1 day" />
                <FormSelectOption value="7d" label="7 days" />
                <FormSelectOption value="30d" label="30 days" />
                <FormSelectOption value="90d" label="90 days" />
                <FormSelectOption value="365d" label="1 year" />
              </FormSelect>
            </FormGroup>
            {createKeyMutation.isError && (
              <p style={{ color: 'red' }}>Failed to create API key.</p>
            )}
          </Form>
        )}
      </Modal>
    </PageSection>
  );
}
