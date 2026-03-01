import { useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Tabs, Tab, TabTitleText,
  Card, CardBody, DescriptionList, DescriptionListGroup,
  DescriptionListTerm, DescriptionListDescription, Label, Spinner,
  Button, Modal, ModalVariant, Form, FormGroup, TextInput,
  ClipboardCopy,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useState } from 'react';
import {
  fetchGroup, fetchGroupMembers, fetchActivityLogs,
  addGroupMember, removeMember, updateMemberRole,
  createServiceAccount,
} from '../api/resources';
import { useAuth } from '../context/AuthContext';
import type { User } from '../types/api';

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
  const [createdSA, setCreatedSA] = useState<User | null>(null);

  // Add member state
  const [isAddMemberOpen, setIsAddMemberOpen] = useState(false);
  const [addMemberUserId, setAddMemberUserId] = useState('');
  const [addMemberRole, setAddMemberRole] = useState('member');

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

  const createSAMutation = useMutation({
    mutationFn: () => {
      const domains = saDomains.trim() ? saDomains.split(',').map(d => d.trim()).filter(Boolean) : undefined;
      return createServiceAccount(id!, {
        username: saUsername,
        email: saEmail || undefined,
        allowed_domains: domains,
      });
    },
    onSuccess: (data) => {
      setCreatedSA(data);
      queryClient.invalidateQueries({ queryKey: ['group-members', id] });
      setSaUsername('');
      setSaEmail('');
      setSaDomains('');
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

  if (isLoading || !group) return <PageSection><Spinner size="xl" /></PageSection>;

  const serviceAccounts = members?.filter(m => m.email?.endsWith('@smtp.internal') || false) || [];
  const humanMembers = members?.filter(m => !m.email?.endsWith('@smtp.internal')) || [];

  return (
    <PageSection>
      <Title headingLevel="h1" size="lg" style={{ marginBottom: '1rem' }}>
        Group: {group.name}
      </Title>

      <Tabs activeKey={activeTab} onSelect={(_e, k) => setActiveTab(k as number)}>
        <Tab eventKey={0} title={<TabTitleText>Details</TabTitleText>}>
          <Card style={{ marginTop: '1rem' }}>
            <CardBody>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>ID</DescriptionListTerm>
                  <DescriptionListDescription>{group.id}</DescriptionListDescription>
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
                          <div style={{ display: 'flex', gap: '0.5rem' }}>
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
                <Thead><Tr><Th>Username</Th><Th>Email</Th><Th>Role</Th><Th>Joined</Th>{isOwnerOrAdmin && <Th>Actions</Th>}</Tr></Thead>
                <Tbody>
                  {serviceAccounts.map((m) => (
                    <Tr key={m.id}>
                      <Td>{m.username || '-'}</Td>
                      <Td>{m.email || m.user_id}</Td>
                      <Td><Label>{m.role}</Label></Td>
                      <Td>{new Date(m.created_at).toLocaleDateString()}</Td>
                      {isOwnerOrAdmin && (
                        <Td>
                          <Button variant="danger" size="sm" onClick={() => { if (confirm('Remove this service account?')) removeMemberMutation.mutate(m.user_id); }}>
                            Remove
                          </Button>
                        </Td>
                      )}
                    </Tr>
                  ))}
                  {serviceAccounts.length === 0 && (
                    <Tr><Td colSpan={isOwnerOrAdmin ? 5 : 4}>No service accounts</Td></Tr>
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
        onClose={() => { setIsCreateSAOpen(false); setCreatedSA(null); }}
        actions={createdSA ? [
          <Button key="close" onClick={() => { setIsCreateSAOpen(false); setCreatedSA(null); }}>Close</Button>,
        ] : [
          <Button key="create" onClick={() => createSAMutation.mutate()} isDisabled={!saUsername || createSAMutation.isPending}>
            {createSAMutation.isPending ? 'Creating...' : 'Create'}
          </Button>,
          <Button key="cancel" variant="link" onClick={() => setIsCreateSAOpen(false)}>Cancel</Button>,
        ]}
      >
        {createdSA ? (
          <div>
            <p style={{ marginBottom: '1rem' }}>Service account created. Copy the API key below - it will not be shown again.</p>
            <FormGroup label="API Key" fieldId="sa-api-key">
              <ClipboardCopy isReadOnly>{createdSA.api_key || ''}</ClipboardCopy>
            </FormGroup>
          </div>
        ) : (
          <Form>
            <FormGroup label="Username" isRequired fieldId="sa-username">
              <TextInput id="sa-username" value={saUsername} onChange={(_e, v) => setSaUsername(v)} isRequired />
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
        )}
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
    </PageSection>
  );
}
