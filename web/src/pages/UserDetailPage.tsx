import { useParams } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import {
  PageSection, Title, Card, CardBody, DescriptionList, DescriptionListGroup,
  DescriptionListTerm, DescriptionListDescription, Label, Spinner,
  Button, Modal, ModalVariant,
  Form, FormGroup, TextInput,
} from '@patternfly/react-core';
import { fetchUser, resetUserPassword } from '../api/resources';

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [isResetOpen, setIsResetOpen] = useState(false);
  const [newPassword, setNewPassword] = useState('');

  const { data: user, isLoading } = useQuery({
    queryKey: ['user', id],
    queryFn: () => fetchUser(id!),
    enabled: !!id,
  });

  const resetMutation = useMutation({
    mutationFn: () => resetUserPassword(id!, newPassword),
    onSuccess: () => {
      setIsResetOpen(false);
      setNewPassword('');
    },
  });

  if (isLoading || !user) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <Title headingLevel="h1" size="lg">User: {user.email}</Title>
        {user.account_type === 'user' && (
          <Button variant="secondary" onClick={() => setIsResetOpen(true)}>Reset Password</Button>
        )}
      </div>

      <Card>
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
    </PageSection>
  );
}
