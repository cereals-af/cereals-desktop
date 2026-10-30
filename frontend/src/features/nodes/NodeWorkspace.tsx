import { nodeQuantity } from './quantities';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Descriptions, Drawer, Space, Spin, Table, Tabs, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { NodeSnapshot, Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { FormSession } from '../resources/FormSession';
import { ManifestViewer } from '../resources/ManifestViewer';
import { ManifestEditor } from '../resources/ManifestEditor';
import { NodePods } from './NodePods';
import { DrainWorkspace } from './DrainWorkspace';

interface Props {
  target: Ref;
  profile: Profile;
  onClose: () => void;
  onSuccess: () => void;
}
export function NodeWorkspace({ target, profile, onClose, onSuccess }: Props) {
  const api = useAPI();
  const [action, setAction] = useState<'cordon' | 'drain' | 'yaml' | 'edit' | null>(null);
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'node', target],
    queryFn: ({ signal }) => api.node(target, signal),
    retry: false,
    gcTime: 0,
  });
  const refresh = () => {
    void query.refetch();
    onSuccess();
  };
  return (
    <Drawer
      open
      size={1180}
      destroyOnHidden
      onClose={onClose}
      title={
        <div className="drawer-title">
          <span>节点 · {target.name}</span>
          <small>
            {profile.context} · {profile.server}
          </small>
        </div>
      }
    >
      <Space wrap className="mutation-toolbar">
        <Button onClick={refresh} loading={query.isFetching}>
          刷新节点
        </Button>
        <Button onClick={() => setAction('yaml')}>节点 YAML</Button>
        <Button onClick={() => setAction('edit')}>编辑节点 YAML</Button>
        <Button disabled={!query.data} onClick={() => setAction('cordon')}>
          {query.data?.unschedulable ? '解封调度' : '封锁调度'}
        </Button>
        <Button danger disabled={!query.data} onClick={() => setAction('drain')}>
          排空节点
        </Button>
      </Space>
      {query.isError && <Alert showIcon type="error" title={errorMessage(query.error)} />}
      {query.isPending && <Spin />}
      {query.data && (
        <Tabs
          items={[
            { key: 'details', label: '节点详情', children: <NodeDetails snapshot={query.data} /> },
            {
              key: 'pods',
              label: '节点上的 Pod',
              children: <NodePods target={target} profile={profile} />,
            },
          ]}
        />
      )}
      {action === 'cordon' && query.data && (
        <Drawer
          open
          size={1080}
          className="manifest-drawer"
          onClose={() => setAction(null)}
          title={`${query.data.unschedulable ? '解封' : '封锁'}调度 · ${target.name}`}
          destroyOnHidden
        >
          <FormSession
            initial={!query.data.unschedulable}
            baseline={query.data.baseline}
            creating={false}
            prepare={(unschedulable, baseline, signal) =>
              api.previewCordon({ target, baseline, unschedulable }, signal)
            }
            readBaseline={async (signal) => (await api.node(target, signal)).baseline}
            onSuccess={refresh}
            onDone={() => setAction(null)}
            onBack={() => setAction(null)}
            doneLabel="返回节点详情"
          >
            {(unschedulable) => (
              <Alert
                showIcon
                type="info"
                title={unschedulable ? '将节点设为不可调度' : '允许节点重新接收调度'}
                description={
                  unschedulable
                    ? '已有 Pod 会继续运行；如需迁移，请使用排空节点。'
                    : '维护完成后恢复调度，由调度器决定后续 Pod 放置。'
                }
              />
            )}
          </FormSession>
        </Drawer>
      )}
      {action === 'drain' && query.data && (
        <DrainWorkspace
          target={target}
          snapshot={query.data}
          profile={profile}
          onClose={() => setAction(null)}
          onSuccess={refresh}
        />
      )}
      {action === 'yaml' && (
        <ManifestViewer
          resource={target}
          contextName={profile.context}
          onClose={() => setAction(null)}
          onEdit={() => setAction('edit')}
        />
      )}
      {action === 'edit' && (
        <ManifestEditor
          target={target}
          profile={profile}
          type={{
            group: '',
            version: 'v1',
            resource: 'nodes',
            kind: 'Node',
            namespaced: false,
            verbs: ['get', 'patch'],
          }}
          onClose={() => setAction(null)}
          onSuccess={refresh}
        />
      )}
    </Drawer>
  );
}
function NodeDetails({ snapshot }: { snapshot: NodeSnapshot }) {
  const capacity = Object.entries(snapshot.capacity ?? {}).map(([name, value]) => ({
    name,
    value: nodeQuantity(name, value ?? '—'),
    allocatable: nodeQuantity(name, snapshot.allocatable?.[name] ?? '—'),
  }));
  return (
    <div className="node-details">
      <Descriptions
        bordered
        size="small"
        column={3}
        items={[
          { key: 'roles', label: '角色', children: snapshot.roles?.join(', ') || '—' },
          {
            key: 'schedule',
            label: '调度',
            children: (
              <Tag color={snapshot.unschedulable ? 'orange' : 'green'}>
                {snapshot.unschedulable ? '已封锁' : '可调度'}
              </Tag>
            ),
          },
          ...Object.entries(snapshot.addresses ?? {}).map(([key, value]) => ({
            key: `address-${key}`,
            label: key,
            children: value,
          })),
          ...Object.entries(snapshot.info ?? {}).map(([key, value]) => ({
            key: `info-${key}`,
            label: key,
            children: value,
          })),
        ]}
      />
      <h3>容量与可分配量</h3>
      <Table
        rowKey="name"
        size="small"
        pagination={false}
        dataSource={capacity}
        columns={[
          { title: '资源', dataIndex: 'name' },
          { title: '容量', dataIndex: 'value' },
          { title: '可分配', dataIndex: 'allocatable' },
        ]}
      />
      <h3>Conditions</h3>
      <Table
        rowKey="type"
        size="small"
        pagination={false}
        dataSource={snapshot.conditions ?? []}
        scroll={{ x: 800 }}
        columns={[
          { title: '类型', dataIndex: 'type' },
          { title: '状态', dataIndex: 'status' },
          { title: '原因', dataIndex: 'reason' },
          { title: '说明', dataIndex: 'message' },
          { title: '最后变化', dataIndex: 'updated' },
        ]}
      />
      <h3>污点</h3>
      <Space wrap>
        {snapshot.taints?.length
          ? snapshot.taints.map((taint) => (
              <Tag key={`${taint.key}/${taint.effect}`}>
                {taint.key}
                {taint.value ? `=${taint.value}` : ''}:{taint.effect}
              </Tag>
            ))
          : '无污点'}
      </Space>
      <h3>标签</h3>
      <Space wrap>
        {Object.entries(snapshot.labels ?? {}).map(([key, value]) => (
          <Tag key={key}>
            {key}={value}
          </Tag>
        ))}
      </Space>
    </div>
  );
}
