import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import {
  PageSection, Title, Card, CardBody,
  Form, FormGroup, TextInput, Switch, Button, ActionGroup, Spinner,
  TextArea,
} from '@patternfly/react-core';
import { fetchRoutingRule, fetchProviders, createRoutingRule, updateRoutingRule } from '../api/resources';
import { FormSelect, FormSelectOption } from '@patternfly/react-core';

export default function RoutingRuleFormPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [priority, setPriority] = useState('100');
  const [providerId, setProviderId] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [conditionsJson, setConditionsJson] = useState('{}');

  const { data: existing, isLoading } = useQuery({
    queryKey: ['routing-rule', id],
    queryFn: () => fetchRoutingRule(id!),
    enabled: isEdit,
  });

  const { data: providers } = useQuery({
    queryKey: ['providers'],
    queryFn: () => fetchProviders(),
  });

  useEffect(() => {
    if (existing) {
      setPriority(String(existing.priority));
      setProviderId(existing.provider_id);
      setEnabled(existing.enabled);
      setConditionsJson(JSON.stringify(existing.conditions, null, 2));
    }
  }, [existing]);

  const saveMutation = useMutation({
    mutationFn: () => {
      let conditions: Record<string, unknown> = {};
      try { conditions = JSON.parse(conditionsJson); } catch { /* keep empty */ }
      const payload = {
        priority: parseInt(priority) || 100,
        provider_id: providerId,
        enabled,
        conditions,
      };
      return isEdit ? updateRoutingRule(id!, payload) : createRoutingRule(payload);
    },
    onSuccess: () => navigate('/routing-rules'),
  });

  if (isEdit && isLoading) return <PageSection><Spinner size="xl" /></PageSection>;

  return (
    <PageSection>
      <Title headingLevel="h1" size="lg" style={{ marginBottom: '1rem' }}>
        {isEdit ? 'Edit Routing Rule' : 'Add Routing Rule'}
      </Title>

      <Card>
        <CardBody>
          <Form>
            <FormGroup label="Priority" isRequired fieldId="rule-priority">
              <TextInput id="rule-priority" type="number" value={priority} onChange={(_e, v) => setPriority(v)} isRequired />
            </FormGroup>
            <FormGroup label="Provider" isRequired fieldId="rule-provider">
              <FormSelect id="rule-provider" value={providerId} onChange={(_e, v) => setProviderId(v)}>
                <FormSelectOption value="" label="-- Select provider --" isDisabled />
                {providers?.map((p) => (
                  <FormSelectOption key={p.id} value={p.id} label={p.name} />
                ))}
              </FormSelect>
            </FormGroup>
            <FormGroup label="Enabled" fieldId="rule-enabled">
              <Switch id="rule-enabled" isChecked={enabled} onChange={(_e, v) => setEnabled(v)} />
            </FormGroup>
            <FormGroup label="Conditions (JSON)" fieldId="rule-conditions">
              <TextArea
                id="rule-conditions"
                value={conditionsJson}
                onChange={(_e, v) => setConditionsJson(v)}
                rows={6}
                style={{ fontFamily: 'monospace' }}
              />
            </FormGroup>

            <ActionGroup>
              <Button onClick={() => saveMutation.mutate()} isDisabled={!providerId || saveMutation.isPending}>
                {saveMutation.isPending ? 'Saving...' : (isEdit ? 'Update' : 'Create')}
              </Button>
              <Button variant="link" onClick={() => navigate('/routing-rules')}>Cancel</Button>
            </ActionGroup>
          </Form>
        </CardBody>
      </Card>
    </PageSection>
  );
}
