import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  PageSection, Title, Card, CardTitle, CardBody,
  Form, FormGroup, TextInput, Button, ActionGroup,
  Alert,
  DescriptionList, DescriptionListGroup, DescriptionListTerm, DescriptionListDescription,
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
      <Title headingLevel="h1" size="lg" className="page-title">
        Settings
      </Title>

      <Card className="card-spaced">
        <CardTitle>Profile</CardTitle>
        <CardBody>
          <DescriptionList>
            <DescriptionListGroup>
              <DescriptionListTerm>Email</DescriptionListTerm>
              <DescriptionListDescription>{me?.user.email}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Username</DescriptionListTerm>
              <DescriptionListDescription>{me?.user.username || '-'}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Account Type</DescriptionListTerm>
              <DescriptionListDescription>{me?.user.account_type}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Current Group</DescriptionListTerm>
              <DescriptionListDescription>{me?.memberships?.find(m => m.group_id === me.current_group.group_id)?.group_name || '-'}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Role</DescriptionListTerm>
              <DescriptionListDescription>{me?.current_group.role}</DescriptionListDescription>
            </DescriptionListGroup>
          </DescriptionList>
        </CardBody>
      </Card>

      {me?.user.account_type === 'user' && (
        <Card>
          <CardTitle>Change Password</CardTitle>
          <CardBody>
            {success && <Alert variant="success" title={success} />}
            {passwordMutation.isError && (
              <Alert variant="danger" title="Failed to change password" />
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
                {passwordMismatch && <p className="field-error">Passwords do not match</p>}
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
