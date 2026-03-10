import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  PageSection, Title, Card, CardBody,
  Form, FormGroup, TextInput, FormSelect, FormSelectOption,
  Switch, Button, ActionGroup, Spinner, Label,
} from '@patternfly/react-core';
import {
  fetchProvider, createProvider, updateProvider,
  fetchProviderAccess, grantProviderAccess, revokeProviderAccess,
  fetchGroups, fetchProviderUsage,
} from '../api/resources';

interface SmtpConfig {
  host: string;
  port: string;
  username: string;
  password: string;
  encryption: string;
}

interface MsGraphConfig {
  tenant_id: string;
  client_id: string;
  client_secret: string;
  user_id: string;
}

interface ApiKeyConfig {
  api_key: string;
  region: string;
  domain: string;
}

export default function ProviderFormPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [name, setName] = useState('');
  const [providerType, setProviderType] = useState('smtp');
  const [enabled, setEnabled] = useState(true);
  const [visibility, setVisibility] = useState('private');
  const [smtp, setSmtp] = useState<SmtpConfig>({ host: '', port: '587', username: '', password: '', encryption: 'starttls' });
  const [msgraph, setMsgraph] = useState<MsGraphConfig>({ tenant_id: '', client_id: '', client_secret: '', user_id: '' });
  const [apiKey, setApiKey] = useState<ApiKeyConfig>({ api_key: '', region: 'us-east-1', domain: '' });

  const { data: existing, isLoading } = useQuery({
    queryKey: ['provider', id],
    queryFn: () => fetchProvider(id!),
    enabled: isEdit,
  });

  useEffect(() => {
    if (existing) {
      setName(existing.name);
      setProviderType(existing.provider_type);
      setEnabled(existing.enabled);
      setVisibility(existing.visibility || 'private');
      const cfg = existing.smtp_config || {};
      setSmtp({
        host: (cfg.host as string) || '',
        port: String(cfg.port || '587'),
        username: (cfg.username as string) || '',
        password: (cfg.password as string) || '',
        encryption: (cfg.encryption as string) || 'starttls',
      });
      setMsgraph({
        tenant_id: (cfg.tenant_id as string) || '',
        client_id: (cfg.client_id as string) || '',
        client_secret: (cfg.client_secret as string) || '',
        user_id: (cfg.user_id as string) || '',
      });
      setApiKey({
        api_key: (cfg.api_key as string) || '',
        region: (cfg.region as string) || 'us-east-1',
        domain: (cfg.domain as string) || '',
      });
    }
  }, [existing]);

  const buildPayload = () => {
    let smtpConfig: Record<string, unknown> = {};
    switch (providerType) {
      case 'smtp':
        smtpConfig = {
          host: smtp.host,
          port: parseInt(smtp.port) || 587,
          username: smtp.username,
          password: smtp.password,
          encryption: smtp.encryption,
        };
        break;
      case 'msgraph':
        smtpConfig = {
          tenant_id: msgraph.tenant_id,
          client_id: msgraph.client_id,
          client_secret: msgraph.client_secret,
          user_id: msgraph.user_id,
        };
        break;
      case 'ses':
        smtpConfig = { api_key: apiKey.api_key, region: apiKey.region };
        break;
      case 'sendgrid':
        smtpConfig = { api_key: apiKey.api_key };
        break;
      case 'mailgun':
        smtpConfig = { api_key: apiKey.api_key, domain: apiKey.domain };
        break;
    }
    return { name, provider_type: providerType, enabled, visibility, smtp_config: smtpConfig };
  };

  const isFormValid = (): boolean => {
    if (!name) return false;
    switch (providerType) {
      case 'smtp': return !!smtp.host;
      case 'msgraph': return !!msgraph.tenant_id && !!msgraph.client_id && !!msgraph.client_secret && !!msgraph.user_id;
      case 'ses': return !!apiKey.api_key && !!apiKey.region;
      case 'sendgrid': return !!apiKey.api_key;
      case 'mailgun': return !!apiKey.api_key && !!apiKey.domain;
      default: return true;
    }
  };

  const queryClient = useQueryClient();
  const [grantGroupId, setGrantGroupId] = useState('');
  const [pendingGroupIds, setPendingGroupIds] = useState<string[]>([]);

  const { data: usageList } = useQuery({
    queryKey: ['provider-usage', id],
    queryFn: () => fetchProviderUsage(id!),
    enabled: isEdit,
  });

  const { data: accessList } = useQuery({
    queryKey: ['provider-access', id],
    queryFn: () => fetchProviderAccess(id!),
    enabled: isEdit && visibility === 'shared',
  });

  const { data: allGroups } = useQuery({
    queryKey: ['groups'],
    queryFn: fetchGroups,
    enabled: visibility === 'shared',
  });

  const grantMutation = useMutation({
    mutationFn: () => grantProviderAccess(id!, grantGroupId),
    onSuccess: () => {
      setGrantGroupId('');
      queryClient.invalidateQueries({ queryKey: ['provider-access', id] });
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (groupId: string) => revokeProviderAccess(id!, groupId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['provider-access', id] }),
  });

  const grantableGroups = allGroups?.filter(
    (g) => g.id !== existing?.group_id
      && !accessList?.some((a) => a.group_id === g.id)
      && !pendingGroupIds.includes(g.id)
  ) ?? [];

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildPayload();
      if (isEdit) {
        return updateProvider(id!, payload);
      }
      const created = await createProvider(payload);
      if (pendingGroupIds.length > 0) {
        await Promise.all(pendingGroupIds.map((gid) => grantProviderAccess(created.id, gid)));
      }
      return created;
    },
    onSuccess: () => navigate('/providers'),
  });

  if (isEdit && isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <Title headingLevel="h1" size="lg" style={{ marginBottom: '1rem' }}>
        {isEdit ? `Edit Provider: ${existing?.name}` : 'Add Provider'}
      </Title>

      <Card>
        <CardBody>
          <Form>
            <FormGroup label="Name" isRequired fieldId="provider-name">
              <TextInput id="provider-name" value={name} onChange={(_e, v) => setName(v)} isRequired />
            </FormGroup>
            <FormGroup label="Type" fieldId="provider-type">
              <FormSelect id="provider-type" value={providerType} onChange={(_e, v) => setProviderType(v)}>
                <FormSelectOption value="smtp" label="SMTP" />
                <FormSelectOption value="ses" label="Amazon SES" />
                <FormSelectOption value="sendgrid" label="SendGrid" />
                <FormSelectOption value="mailgun" label="Mailgun" />
                <FormSelectOption value="msgraph" label="Microsoft Graph" />
                <FormSelectOption value="stdout" label="Stdout" />
              </FormSelect>
            </FormGroup>
            <FormGroup label="Enabled" fieldId="provider-enabled">
              <Switch id="provider-enabled" isChecked={enabled} onChange={(_e, v) => setEnabled(v)} />
            </FormGroup>
            <FormGroup label="Visibility" fieldId="provider-visibility">
              <FormSelect id="provider-visibility" value={visibility} onChange={(_e, v) => setVisibility(v)}>
                <FormSelectOption value="private" label="Private (owner group only)" />
                <FormSelectOption value="shared" label="Shared (granted groups)" />
                <FormSelectOption value="global" label="Global (all groups)" />
              </FormSelect>
            </FormGroup>

            {providerType === 'smtp' && (
              <>
                <Title headingLevel="h3" size="md" style={{ marginTop: '1rem', marginBottom: '0.5rem' }}>
                  SMTP Configuration
                </Title>
                <FormGroup label="Host" isRequired fieldId="smtp-host">
                  <TextInput id="smtp-host" value={smtp.host} onChange={(_e, v) => setSmtp({ ...smtp, host: v })} isRequired />
                </FormGroup>
                <FormGroup label="Port" fieldId="smtp-port">
                  <TextInput id="smtp-port" type="number" value={smtp.port} onChange={(_e, v) => setSmtp({ ...smtp, port: v })} />
                </FormGroup>
                <FormGroup label="Username" fieldId="smtp-username">
                  <TextInput id="smtp-username" value={smtp.username} onChange={(_e, v) => setSmtp({ ...smtp, username: v })} />
                </FormGroup>
                <FormGroup label="Password" fieldId="smtp-password">
                  <TextInput id="smtp-password" type="password" value={smtp.password} onChange={(_e, v) => setSmtp({ ...smtp, password: v })} />
                </FormGroup>
                <FormGroup label="Encryption" fieldId="smtp-encryption">
                  <FormSelect id="smtp-encryption" value={smtp.encryption} onChange={(_e, v) => setSmtp({ ...smtp, encryption: v })}>
                    <FormSelectOption value="none" label="None" />
                    <FormSelectOption value="starttls" label="STARTTLS" />
                    <FormSelectOption value="tls" label="TLS/SSL" />
                  </FormSelect>
                </FormGroup>
              </>
            )}

            {providerType === 'msgraph' && (
              <>
                <Title headingLevel="h3" size="md" style={{ marginTop: '1rem', marginBottom: '0.5rem' }}>
                  Microsoft Graph Configuration
                </Title>
                <FormGroup label="Azure AD Tenant ID" isRequired fieldId="ms-tenant-id">
                  <TextInput id="ms-tenant-id" value={msgraph.tenant_id} onChange={(_e, v) => setMsgraph({ ...msgraph, tenant_id: v })} isRequired />
                </FormGroup>
                <FormGroup label="Application Client ID" isRequired fieldId="ms-client-id">
                  <TextInput id="ms-client-id" value={msgraph.client_id} onChange={(_e, v) => setMsgraph({ ...msgraph, client_id: v })} isRequired />
                </FormGroup>
                <FormGroup label="Client Secret" isRequired fieldId="ms-client-secret">
                  <TextInput id="ms-client-secret" type="password" value={msgraph.client_secret} onChange={(_e, v) => setMsgraph({ ...msgraph, client_secret: v })} isRequired />
                </FormGroup>
                <FormGroup label="User ID / UPN (Microsoft 365 user or email)" isRequired fieldId="ms-user-id">
                  <TextInput id="ms-user-id" value={msgraph.user_id} onChange={(_e, v) => setMsgraph({ ...msgraph, user_id: v })} isRequired />
                </FormGroup>
              </>
            )}

            {(providerType === 'sendgrid' || providerType === 'ses' || providerType === 'mailgun') && (
              <>
                <Title headingLevel="h3" size="md" style={{ marginTop: '1rem', marginBottom: '0.5rem' }}>
                  {providerType === 'ses' ? 'Amazon SES' : providerType === 'sendgrid' ? 'SendGrid' : 'Mailgun'} Configuration
                </Title>
                <FormGroup label="API Key" isRequired fieldId="api-key">
                  <TextInput id="api-key" type="password" value={apiKey.api_key} onChange={(_e, v) => setApiKey({ ...apiKey, api_key: v })} isRequired />
                </FormGroup>
                {providerType === 'ses' && (
                  <FormGroup label="Region" isRequired fieldId="ses-region">
                    <TextInput id="ses-region" value={apiKey.region} onChange={(_e, v) => setApiKey({ ...apiKey, region: v })} isRequired />
                  </FormGroup>
                )}
                {providerType === 'mailgun' && (
                  <FormGroup label="Sending Domain" isRequired fieldId="mg-domain">
                    <TextInput id="mg-domain" value={apiKey.domain} onChange={(_e, v) => setApiKey({ ...apiKey, domain: v })} isRequired />
                  </FormGroup>
                )}
              </>
            )}

            {visibility === 'shared' && (
              <>
                <Title headingLevel="h3" size="md" style={{ marginTop: '1.5rem', marginBottom: '0.5rem' }}>
                  Group Access
                </Title>
                <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
                  <FormSelect
                    id="grant-group"
                    value={grantGroupId}
                    onChange={(_e, v) => setGrantGroupId(v)}
                    style={{ maxWidth: '300px' }}
                  >
                    <FormSelectOption value="" label="-- Select group --" isDisabled />
                    {grantableGroups.map((g) => (
                      <FormSelectOption key={g.id} value={g.id} label={g.name} />
                    ))}
                  </FormSelect>
                  <Button
                    variant="secondary"
                    isDisabled={!grantGroupId}
                    isLoading={isEdit && grantMutation.isPending}
                    onClick={() => {
                      if (isEdit) {
                        grantMutation.mutate();
                      } else {
                        setPendingGroupIds([...pendingGroupIds, grantGroupId]);
                        setGrantGroupId('');
                      }
                    }}
                  >
                    Grant
                  </Button>
                </div>
                {/* Edit mode: show persisted access list */}
                {isEdit && accessList && accessList.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', marginBottom: '0.5rem' }}>
                    {accessList.map((a) => {
                      const groupName = allGroups?.find((g) => g.id === a.group_id)?.name ?? a.group_id;
                      return (
                        <Label
                          key={a.group_id}
                          color="blue"
                          onClose={() => revokeMutation.mutate(a.group_id)}
                        >
                          {groupName}
                        </Label>
                      );
                    })}
                  </div>
                )}
                {/* Create mode: show pending groups */}
                {!isEdit && pendingGroupIds.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                    {pendingGroupIds.map((gid) => {
                      const groupName = allGroups?.find((g) => g.id === gid)?.name ?? gid;
                      return (
                        <Label
                          key={gid}
                          color="blue"
                          onClose={() => setPendingGroupIds(pendingGroupIds.filter((x) => x !== gid))}
                        >
                          {groupName}
                        </Label>
                      );
                    })}
                  </div>
                )}
                {isEdit && (!accessList || accessList.length === 0) && (
                  <p style={{ color: '#6a6e73' }}>No groups have been granted access yet.</p>
                )}
                {!isEdit && pendingGroupIds.length === 0 && (
                  <p style={{ color: '#6a6e73' }}>Select groups to grant access on creation.</p>
                )}
              </>
            )}

            {isEdit && (
              <>
                <Title headingLevel="h3" size="md" style={{ marginTop: '1.5rem', marginBottom: '0.5rem' }}>
                  Usage
                </Title>
                {usageList && usageList.length > 0 ? (
                  <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                    <thead>
                      <tr style={{ borderBottom: '2px solid #d2d2d2', textAlign: 'left' }}>
                        <th style={{ padding: '0.5rem' }}>Group</th>
                        <th style={{ padding: '0.5rem' }}>Email</th>
                        <th style={{ padding: '0.5rem' }}>Account Type</th>
                        <th style={{ padding: '0.5rem' }}>Role</th>
                      </tr>
                    </thead>
                    <tbody>
                      {usageList.map((u) => (
                        <tr key={`${u.group_id}-${u.user_id}`} style={{ borderBottom: '1px solid #d2d2d2' }}>
                          <td style={{ padding: '0.5rem' }}>{u.group_name}</td>
                          <td style={{ padding: '0.5rem' }}>{u.email}</td>
                          <td style={{ padding: '0.5rem' }}>{u.account_type}</td>
                          <td style={{ padding: '0.5rem' }}>{u.role}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <p style={{ color: '#6a6e73' }}>No users assigned to this provider.</p>
                )}
              </>
            )}

            <ActionGroup>
              <Button onClick={() => saveMutation.mutate()} isDisabled={!isFormValid() || saveMutation.isPending}>
                {saveMutation.isPending ? 'Saving...' : (isEdit ? 'Update' : 'Create')}
              </Button>
              <Button variant="link" onClick={() => navigate('/providers')}>Cancel</Button>
            </ActionGroup>
          </Form>
        </CardBody>
      </Card>
    </PageSection>
  );
}
