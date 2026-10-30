import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Space, Table } from 'antd';
import { useAPI } from '../../api/context';
import type { NodePod, Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { NamespaceSelect } from '../resources/NamespaceSelect';
import { ManifestViewer } from '../resources/ManifestViewer';
import { UsageCell } from '../metrics/UsageCell';
import { usageKey, useResourceUsage } from '../metrics/useResourceUsage';
import { useTableColumns } from '../resources/useTableColumns';
import { TableColumnSettings } from '../resources/TableColumnSettings';

export function NodePods({ target, profile }: { target: Ref; profile: Profile }) {
  const api = useAPI();
  const settings = useTableColumns('node-pods');
  const [namespace, setNamespace] = useState('');
  const [cursors, setCursors] = useState(['']);
  const [selected, setSelected] = useState<{ target: Ref; uid: string; tab: string } | null>(null);
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'node-pods', target, namespace, cursors.at(-1)],
    queryFn: ({ signal }) => api.nodePods(target, namespace, cursors.at(-1) ?? '', signal),
    retry: false,
    gcTime: 0,
  });
  const open = (pod: NodePod, tab = 'yaml') =>
    setSelected({
      uid: pod.uid,
      tab,
      target: {
        clusterID: target.clusterID,
        group: '',
        version: 'v1',
        resource: 'pods',
        namespace: pod.namespace,
        name: pod.name,
      },
    });
  const {
    host: usageHost,
    samples: usageSamples,
    reason: usageReason,
    refresh: refreshUsage,
  } = useResourceUsage(
    {
      clusterID: target.clusterID,
      resource: 'pods',
      namespace,
      targets: (query.data?.pods ?? []).map(({ name, namespace, uid }) => ({
        name,
        namespace,
        uid,
      })),
    },
    settings.shown('usage'),
  );
  return (
    <div ref={usageHost} className="node-pods">
      <Space className="mutation-toolbar">
        <NamespaceSelect
          clusterID={target.clusterID}
          value={namespace}
          onChange={(value) => {
            setNamespace(value);
            setCursors(['']);
          }}
        />
        <Button
          onClick={() => {
            setCursors(['']);
            void query.refetch();
            if (settings.shown('usage')) void refreshUsage();
          }}
        >
          刷新 Pod
        </Button>
        <TableColumnSettings
          settings={settings}
          options={[{ key: 'usage', label: 'CPU / 内存用量' }]}
        />
      </Space>
      {query.isError && (
        <Alert
          showIcon
          type="error"
          title={errorMessage(query.error)}
          description="Pod 查询失败不会影响节点详情；可选择有权限的命名空间。"
        />
      )}
      <Table<NodePod>
        rowKey="uid"
        dataSource={query.data?.pods ?? []}
        loading={query.isFetching}
        pagination={false}
        scroll={{ x: 800 }}
        columns={[
          {
            title: 'Pod 操作',
            render: (_, pod) => (
              <Space>
                <Button aria-label="日志" size="small" onClick={() => open(pod, 'operations')}>
                  日志
                </Button>
                <Button aria-label="终端" size="small" onClick={() => open(pod, 'operations')}>
                  终端
                </Button>
              </Space>
            ),
          },
          {
            title: 'Pod',
            dataIndex: 'name',
            render: (_, pod) => (
              <Button type="link" onClick={() => open(pod)}>
                {pod.name}
              </Button>
            ),
          },
          { title: '命名空间', dataIndex: 'namespace' },
          { title: '状态', dataIndex: 'phase' },
          ...(settings.shown('usage')
            ? [
                {
                  title: 'CPU / 内存用量',
                  key: 'usage',
                  width: 170,
                  render: (_: unknown, pod: NodePod) => (
                    <UsageCell sample={usageSamples.get(usageKey(pod))} reason={usageReason} />
                  ),
                },
              ]
            : []),
          { title: '控制器引用', dataIndex: 'owner', render: (value: string) => value || '无' },
          {
            title: '本地数据',
            dataIndex: 'localData',
            render: (values: string[] | null) => values?.join(', ') || '—',
          },
        ]}
      />
      <Space className="mutation-toolbar">
        <Button
          disabled={cursors.length === 1 || query.isFetching}
          onClick={() => setCursors((all) => all.slice(0, -1))}
        >
          上一页 Pod
        </Button>
        <span>第 {cursors.length} 页</span>
        <Button
          disabled={!query.data?.continue || query.isFetching}
          onClick={() => {
            if (query.data?.continue) setCursors([...cursors, query.data.continue]);
          }}
        >
          下一页 Pod
        </Button>
      </Space>
      {selected && (
        <ManifestViewer
          resource={selected.target}
          initialTab={selected.tab}
          expectedUID={selected.tab === 'operations' ? selected.uid : undefined}
          contextName={profile.context}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  );
}
