import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import {
  PageSection, Title, Card, CardBody,
  Form, FormGroup, TextInput, FormSelect, FormSelectOption,
  Switch, Button, ActionGroup, Spinner,
} from '@patternfly/react-core';
import { fetchProvider, createProvider, updateProvider } from '../api/resources';

interface SmtpConfig {
  host: string;
  port: string;
  username: string;
  password: string;
  encryption: string;
}

export default function ProviderFormPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [name, setName] = useState('');
  const [providerType, setProviderType] = useState('smtp');
  const [enabled, setEnabled] = useState(true);
  const [smtp, setSmtp] = useState<SmtpConfig>({ host: '', port: '587', username: '', password: '', encryption: 'starttls' });

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
      const cfg = existing.smtp_config || {};
      setSmtp({
        host: (cfg.host as string) || '',
        port: String(cfg.port || '587'),
        username: (cfg.username as string) || '',
        password: (cfg.password as string) || '',
        encryption: (cfg.encryption as string) || 'starttls',
      });
    }
  }, [existing]);

  const saveMutation = useMutation({
    mutationFn: () => {
      const payload = {
        name,
        provider_type: providerType,
        enabled,
        smtp_config: {
          host: smtp.host,
          port: parseInt(smtp.port) || 587,
          username: smtp.username,
          password: smtp.password,
          encryption: smtp.encryption,
        },
      };
      return isEdit ? updateProvider(id!, payload) : createProvider(payload);
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
              </FormSelect>
            </FormGroup>
            <FormGroup label="Enabled" fieldId="provider-enabled">
              <Switch id="provider-enabled" isChecked={enabled} onChange={(_e, v) => setEnabled(v)} />
            </FormGroup>

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

            <ActionGroup>
              <Button onClick={() => saveMutation.mutate()} isDisabled={!name || !smtp.host || saveMutation.isPending}>
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
