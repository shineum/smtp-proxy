import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  PageSection, Title, Card, CardTitle, CardBody,
  Form, FormGroup, TextInput, Button, ActionGroup,
  Alert,
} from '@patternfly/react-core';
import { changePassword } from '../api/auth';
import { useAuth } from '../context/AuthContext';

export default function SettingsPage() {
  const { me } = useAuth();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [success, setSuccess] = useState('');

  const passwordMutation = useMutation({
    mutationFn: () => changePassword(currentPassword, newPassword),
    onSuccess: () => {
      setSuccess('Password changed successfully');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    },
  });

  const passwordMismatch = confirmPassword !== '' && newPassword !== confirmPassword;

  return (
    <PageSection>
      <Title headingLevel="h1" size="lg" style={{ marginBottom: '1rem' }}>
        Settings
      </Title>

      <Card style={{ marginBottom: '1rem' }}>
        <CardTitle>Profile</CardTitle>
        <CardBody>
          <p><strong>Email:</strong> {me?.user.email}</p>
          <p><strong>Username:</strong> {me?.user.username || '-'}</p>
          <p><strong>Account Type:</strong> {me?.user.account_type}</p>
          <p><strong>Current Group:</strong> {me?.memberships.find(m => m.group_id === me.current_group.group_id)?.group_name || '-'}</p>
          <p><strong>Role:</strong> {me?.current_group.role}</p>
        </CardBody>
      </Card>

      {me?.user.account_type === 'user' && (
        <Card>
          <CardTitle>Change Password</CardTitle>
          <CardBody>
            {success && <Alert variant="success" title={success} style={{ marginBottom: '1rem' }} />}
            {passwordMutation.isError && (
              <Alert variant="danger" title="Failed to change password" style={{ marginBottom: '1rem' }} />
            )}
            <Form>
              <FormGroup label="Current Password" isRequired fieldId="current-password">
                <TextInput
                  id="current-password"
                  type="password"
                  value={currentPassword}
                  onChange={(_e, v) => { setCurrentPassword(v); setSuccess(''); }}
                  isRequired
                />
              </FormGroup>
              <FormGroup label="New Password" isRequired fieldId="new-password">
                <TextInput
                  id="new-password"
                  type="password"
                  value={newPassword}
                  onChange={(_e, v) => { setNewPassword(v); setSuccess(''); }}
                  isRequired
                />
              </FormGroup>
              <FormGroup
                label="Confirm New Password"
                isRequired
                fieldId="confirm-password"
              >
                <TextInput
                  id="confirm-password"
                  type="password"
                  value={confirmPassword}
                  onChange={(_e, v) => { setConfirmPassword(v); setSuccess(''); }}
                  isRequired
                />
                {passwordMismatch && <p style={{ color: '#C9190B', fontSize: '0.875rem', marginTop: '0.25rem' }}>Passwords do not match</p>}
              </FormGroup>
              <ActionGroup>
                <Button
                  onClick={() => passwordMutation.mutate()}
                  isDisabled={!currentPassword || !newPassword || passwordMismatch || passwordMutation.isPending}
                >
                  {passwordMutation.isPending ? 'Changing...' : 'Change Password'}
                </Button>
              </ActionGroup>
            </Form>
          </CardBody>
        </Card>
      )}
    </PageSection>
  );
}
