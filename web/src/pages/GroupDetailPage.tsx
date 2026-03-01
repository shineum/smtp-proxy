import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  PageSection, Title, Tabs, Tab, TabTitleText,
  Card, CardBody, DescriptionList, DescriptionListGroup,
  DescriptionListTerm, DescriptionListDescription, Label, Spinner,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useState } from 'react';
import { fetchGroup, fetchGroupMembers, fetchActivityLogs } from '../api/resources';

export default function GroupDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [activeTab, setActiveTab] = useState(0);

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

  if (isLoading || !group) return <PageSection><Spinner size="xl" /></PageSection>;

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
              <Table aria-label="Members">
                <Thead><Tr><Th>Email</Th><Th>Username</Th><Th>Role</Th><Th>Joined</Th></Tr></Thead>
                <Tbody>
                  {members?.map((m) => (
                    <Tr key={m.id}>
                      <Td>{m.email || m.user_id}</Td>
                      <Td>{m.username || '-'}</Td>
                      <Td><Label>{m.role}</Label></Td>
                      <Td>{new Date(m.created_at).toLocaleDateString()}</Td>
                    </Tr>
                  ))}
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
    </PageSection>
  );
}
