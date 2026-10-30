import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Drawer, Select, Space, Spin, Table } from 'antd';
import { useAPI } from '../../api/context';
import type { Ref, RelatedNode, RelationReport } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { PodConsoles } from '../pods/PodConsoles';
import { usePodConsoles } from '../pods/usePodConsoles';
import { RelationsPanel } from '../relations/RelationsPanel';
import { RelatedYAML } from '../relations/RelatedYAML';
import { UsageCell } from '../metrics/UsageCell';
import { usageKey, useResourceUsage } from '../metrics/useResourceUsage';
import { useTableColumns } from '../resources/useTableColumns';
import { TableColumnSettings } from '../resources/TableColumnSettings';

export function WorkloadPods({ target, contextName }: { target: Ref; contextName: string }) {
  const api = useAPI();
  const settings = useTableColumns('workload-pods');
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'workload-pods', target],
    queryFn: ({ signal }) => api.relations(target, signal),
    retry: false,
    gcTime: 0,
    refetchInterval: 5000,
  });
  return (
    <>
      <div className="relation-toolbar">
        <span>
          {contextName} / {target.namespace} / {target.name} · 通过控制器 UID 查找 Pod
        </span>
        <Space>
          <Button loading={query.isFetching} onClick={() => void query.refetch()}>
            刷新容器组
          </Button>
          <TableColumnSettings
            settings={settings}
            options={[{ key: 'usage', label: 'CPU / 内存用量' }]}
          />
        </Space>
      </div>
      {query.isError && (
        <Alert
          showIcon
          type="error"
          title={errorMessage(query.error)}
          description="关联读取失败，可能缺少父资源、ReplicaSet 或 Pod 读取权限；上次结果可能已过期。"
        />
      )}
      {query.isPending && <Spin />}
      {query.data && (
        <PodList
          key={query.data.root.uid}
          report={query.data}
          contextName={contextName}
          showUsage={settings.shown('usage')}
        />
      )}
    </>
  );
}
function PodList({
  report,
  contextName,
  showUsage,
}: {
  report: RelationReport;
  contextName: string;
  showUsage: boolean;
}) {
  const section = report.sections?.find((s) => s.key === 'pods');
  const pods = section?.items ?? [];
  const consoles = usePodConsoles(pods.length === 1 ? pods[0] : undefined);
  const [yaml, setYAML] = useState<RelatedNode | null>(null);
  const [events, setEvents] = useState<RelatedNode | null>(null);
  const {
    host: usageHost,
    samples: usageSamples,
    reason: usageReason,
  } = useResourceUsage(
    {
      clusterID: report.root.target.clusterID,
      resource: 'pods',
      namespace: report.root.target.namespace,
      targets: pods.map((pod) => ({
        name: pod.target.name,
        namespace: pod.target.namespace,
        uid: pod.uid,
      })),
    },
    showUsage,
  );
  return (
    <div ref={usageHost}>
      {section?.error && <Alert showIcon type="error" title={`Pod 查询失败：${section.error}`} />}
      {section?.truncated && (
        <Alert
          showIcon
          type="warning"
          title="关联结果不完整，仅展示已找到的 Pod；请检查上游 ReplicaSet 的读取权限或缩小资源范围。"
        />
      )}
      {pods.length === 0 && (
        <Alert
          type="info"
          showIcon
          title={
            section?.error || section?.truncated ? '暂时不能判断是否存在 Pod' : '当前没有关联 Pod'
          }
          description="副本为零、尚未调度或控制器未创建 Pod 时无法打开会话。"
        />
      )}
      <Table<RelatedNode>
        rowKey="uid"
        size="small"
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
        dataSource={pods}
        scroll={{ x: 950 }}
        columns={[
          {
            title: 'Pod',
            render: (_, pod) => (
              <Button type="link" className="resource-link" onClick={() => consoles.open(pod)}>
                {pod.target.name}
              </Button>
            ),
          },
          { title: '状态', dataIndex: 'status' },
          ...(showUsage
            ? [
                {
                  title: 'CPU / 内存用量',
                  key: 'usage',
                  width: 170,
                  render: (_: unknown, pod: RelatedNode) => (
                    <UsageCell
                      sample={usageSamples.get(usageKey({ ...pod.target, uid: pod.uid }))}
                      reason={usageReason}
                    />
                  ),
                },
              ]
            : []),
          { title: '就绪', render: (_, pod) => pod.pod?.ready ?? '—' },
          { title: '重启', render: (_, pod) => pod.pod?.restarts ?? '—' },
          { title: '节点', dataIndex: 'detail' },
          { title: '容器', render: (_, pod) => pod.pod?.containers?.join(', ') || '—' },
          {
            title: '操作',
            width: 140,
            render: (_, pod) => (
              <Space wrap>
                <Button aria-label="事件" size="small" onClick={() => setEvents(pod)}>
                  事件
                </Button>
                <Button size="small" onClick={() => setYAML(pod)}>
                  YAML
                </Button>
              </Space>
            ),
          },
        ]}
      />
      {pods.length > 1 && (
        <div className="pod-container-selector">
          <label>选择操作 Pod</label>
          <Select
            aria-label="工作负载 Pod"
            placeholder="选择明确的 Pod，再选择容器"
            showSearch
            value={consoles.active || undefined}
            options={pods.map((p) => ({
              value: p.uid,
              label: `${p.target.name} · ${p.status} · ${p.detail || '未调度'}`,
            }))}
            onChange={(uid) => {
              const pod = pods.find((p) => p.uid === uid);
              if (pod) consoles.open(pod);
            }}
          />
        </div>
      )}
      <PodConsoles value={consoles} />
      {yaml && <RelatedYAML node={yaml} contextName={contextName} onClose={() => setYAML(null)} />}
      {events && (
        <Drawer
          open
          size={1080}
          title={`Pod 事件 · ${events.target.name}`}
          onClose={() => setEvents(null)}
          destroyOnHidden
        >
          <RelationsPanel
            key={events.uid}
            target={events.target}
            contextName={contextName}
            expectedUID={events.uid}
          />
        </Drawer>
      )}
    </div>
  );
}
