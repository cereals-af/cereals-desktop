import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Descriptions, Drawer, Select, Space, Spin, Table, Tabs, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { CustomDefinition, CustomVersion, Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { ManifestViewer } from '../resources/ManifestViewer';
import { ManifestEditor } from '../resources/ManifestEditor';
import { DeleteResource } from '../resources/DeleteResource';
import { CustomInstances } from './CustomInstances';

interface Props {
  target: Ref;
  profile: Profile;
  onClose: () => void;
  onSuccess: () => void;
}
export function CRDWorkspace(props: Props) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', props.target.clusterID, 'crd-definition', props.target],
    queryFn: ({ signal }) => api.customDefinition(props.target, signal),
    gcTime: 0,
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  });
  return (
    <Drawer
      open
      size={1200}
      destroyOnHidden
      onClose={props.onClose}
      title={
        <div className="drawer-title">
          <span>CRD 定义与实例 · {query.data?.name || props.target.resource}</span>
          <small>
            {props.profile.context} · {props.profile.server}
          </small>
        </div>
      }
    >
      <div className="mutation-toolbar">
        <Button loading={query.isFetching} onClick={() => void query.refetch()}>
          刷新 CRD 定义
        </Button>
      </div>
      {query.isPending && <Spin />}
      {query.isError && (
        <Alert
          type="warning"
          showIcon
          title={errorMessage(query.error)}
          description="需要 get customresourcedefinitions 权限；聚合 API 不一定由 CRD 定义。通用资源列表和 YAML 入口仍可使用。"
        />
      )}
      {query.data && (
        <DefinitionDetails
          key={`${query.data.baseline.uid}/${query.data.baseline.resourceVersion}`}
          {...props}
          definition={query.data}
          onChanged={() => {
            void query.refetch();
            props.onSuccess();
          }}
        />
      )}
    </Drawer>
  );
}
function DefinitionDetails(props: Props & { definition: CustomDefinition; onChanged: () => void }) {
  const definition = props.definition;
  const versions = definition.versions ?? [];
  const [version, setVersion] = useState(
    versions.find((item) => item.name === props.target.version && item.served)?.name ??
      versions.find((item) => item.storage && item.served)?.name ??
      versions.find((item) => item.served)?.name ??
      '',
  );
  const [tab, setTab] = useState('definition');
  const [action, setAction] = useState<'yaml' | 'edit' | 'delete' | null>(null);
  const target: Ref = {
    clusterID: props.profile.id,
    group: 'apiextensions.k8s.io',
    version: 'v1',
    resource: 'customresourcedefinitions',
    namespace: '',
    name: definition.name,
  };
  const versionInfo = versions.find((item) => item.name === version);
  return (
    <>
      <Space className="mutation-toolbar">
        <Button onClick={() => setAction('yaml')}>定义 YAML</Button>
        <Button onClick={() => setAction('edit')}>编辑定义 YAML</Button>
        <Button danger onClick={() => setAction('delete')}>
          删除 CRD
        </Button>
      </Space>
      <Descriptions
        items={[
          { key: 'kind', label: 'Kind', children: definition.kind },
          {
            key: 'group',
            label: 'Group / Resource',
            children: `${definition.group} / ${definition.resource}`,
          },
          {
            key: 'scope',
            label: 'Scope',
            children: definition.namespaced ? 'Namespaced' : 'Cluster',
          },
          {
            key: 'stored',
            label: '实际存储版本',
            children: definition.storedVersions?.join(', ') || '尚未报告',
          },
        ]}
      />
      <Tabs
        activeKey={tab}
        onChange={setTab}
        destroyOnHidden
        items={[
          {
            key: 'definition',
            label: '版本与状态',
            children: (
              <div className="crd-overview">
                <Table<CustomVersion>
                  rowKey="name"
                  pagination={false}
                  dataSource={versions}
                  columns={[
                    { title: '版本', dataIndex: 'name' },
                    {
                      title: '提供服务',
                      render: (_, item) => (
                        <Tag color={item.served ? 'green' : 'default'}>
                          {item.served ? 'served' : '未提供服务'}
                        </Tag>
                      ),
                    },
                    {
                      title: '新写入存储',
                      render: (_, item) => (item.storage ? <Tag color="blue">storage</Tag> : '—'),
                    },
                    {
                      title: '弃用状态',
                      render: (_, item) => (item.deprecated ? item.warning || '已弃用' : '—'),
                    },
                    {
                      title: '实例',
                      render: (_, item) => (
                        <Button
                          disabled={!item.served}
                          onClick={() => {
                            setVersion(item.name);
                            setTab('instances');
                          }}
                        >
                          浏览 {item.name} 实例
                        </Button>
                      ),
                    },
                  ]}
                />
                <Table
                  rowKey="type"
                  pagination={false}
                  dataSource={definition.conditions ?? []}
                  locale={{ emptyText: 'CRD 尚未报告 Conditions' }}
                  columns={[
                    { title: 'Condition', dataIndex: 'type' },
                    { title: '状态', dataIndex: 'status' },
                    { title: '原因', dataIndex: 'reason' },
                    { title: '说明', dataIndex: 'message' },
                  ]}
                />
              </div>
            ),
          },
          {
            key: 'instances',
            label: '实例与动态列',
            children: (
              <>
                <div className="mutation-toolbar">
                  <span>API 版本</span>
                  <Select
                    aria-label="CRD API 版本"
                    value={version || undefined}
                    onChange={setVersion}
                    options={versions.map((item) => ({
                      value: item.name,
                      label: `${item.name}${item.storage ? ' · storage' : ''}${item.deprecated ? ' · 已弃用' : ''}`,
                      disabled: !item.served,
                    }))}
                  />
                </div>
                {versionInfo?.deprecated && (
                  <Alert
                    type="warning"
                    title={versionInfo.warning || '此版本已弃用，建议选择其他 served 版本'}
                  />
                )}
                {version ? (
                  <CustomInstances
                    key={version}
                    profile={props.profile}
                    definition={definition}
                    version={version}
                  />
                ) : (
                  <Alert type="warning" title="当前没有 served 版本" />
                )}
                <h3>additionalPrinterColumns · {version}</h3>
                <Table
                  rowKey={(item) => `${item.name}/${item.jsonPath}`}
                  dataSource={versionInfo?.columns ?? []}
                  pagination={false}
                  locale={{ emptyText: '未配置自定义列，显示 API Server 默认列' }}
                  columns={[
                    { title: '名称', dataIndex: 'name' },
                    { title: '类型', dataIndex: 'type' },
                    { title: 'JSONPath', dataIndex: 'jsonPath' },
                    { title: '优先级', dataIndex: 'priority' },
                  ]}
                />
              </>
            ),
          },
        ]}
      />
      {action === 'yaml' && (
        <ManifestViewer
          resource={target}
          contextName={props.profile.context}
          onClose={() => setAction(null)}
          onEdit={() => setAction('edit')}
          onDelete={() => setAction('delete')}
        />
      )}
      {action === 'edit' && (
        <ManifestEditor
          target={target}
          type={{ ...target, kind: 'CustomResourceDefinition', namespaced: false, verbs: [] }}
          profile={props.profile}
          onClose={() => setAction(null)}
          onSuccess={props.onChanged}
        />
      )}
      {action === 'delete' && (
        <DeleteResource
          target={target}
          profile={props.profile}
          onClose={() => setAction(null)}
          onSuccess={props.onChanged}
        />
      )}
    </>
  );
}
