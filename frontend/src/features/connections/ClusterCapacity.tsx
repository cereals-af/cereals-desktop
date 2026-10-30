import { Alert, Table } from 'antd';
import type { ClusterOverview } from '../../api/types';
import { nodeQuantity } from '../nodes/quantities';

const resourceLabels: Record<string, string> = {
  cpu: 'CPU',
  memory: '内存',
  'ephemeral-storage': '临时存储',
  pods: 'Pod 容量',
};
function capacityValue(name: string, value?: string): string {
  if (value === undefined) return '—';
  if (name === 'cpu')
    return `${Number(value).toLocaleString('zh-CN', { maximumFractionDigits: 3 })} 核`;
  return nodeQuantity(name, value);
}

export function ClusterCapacity({ snapshot }: { snapshot: ClusterOverview }) {
  const nodes = snapshot.nodes;
  const capacities = [
    ...new Set([
      'cpu',
      'memory',
      'ephemeral-storage',
      'pods',
      ...Object.keys(nodes?.capacity ?? {}),
      ...Object.keys(nodes?.allocatable ?? {}),
    ]),
  ];
  return (
    <section className="cluster-overview-section">
      <h3>集群规模</h3>
      <p className="muted">
        节点容量合计，包含不可调度节点；可分配量为扣除系统预留后的额度，不代表当前剩余或实时使用量。
      </p>
      {snapshot.nodeError && (
        <Alert
          showIcon
          type="warning"
          title="无法统计节点与容量"
          description={snapshot.nodeError}
        />
      )}
      <div className="cluster-capacity-grid">
        <CapacityCard
          label="节点总数"
          value={nodes ? String(nodes.count) : '—'}
          detail={
            nodes
              ? `就绪 ${nodes.ready} · 未就绪 ${nodes.count - nodes.ready} · 不可调度 ${nodes.unschedulable}`
              : '节点信息不可用'
          }
        />
        <CapacityCard
          label="CPU 总容量"
          value={capacityValue('cpu', nodes?.capacity?.cpu)}
          detail={`可分配 ${capacityValue('cpu', nodes?.allocatable?.cpu)}`}
        />
        <CapacityCard
          label="内存总容量"
          value={capacityValue('memory', nodes?.capacity?.memory)}
          detail={`可分配 ${capacityValue('memory', nodes?.allocatable?.memory)}`}
        />
        <CapacityCard
          label="临时存储总容量"
          value={capacityValue('ephemeral-storage', nodes?.capacity?.['ephemeral-storage'])}
          detail={`可分配 ${capacityValue('ephemeral-storage', nodes?.allocatable?.['ephemeral-storage'])}`}
        />
      </div>
      {nodes && (
        <Table
          size="small"
          rowKey="name"
          pagination={false}
          dataSource={capacities.map((name) => ({
            name,
            capacity: capacityValue(name, nodes.capacity?.[name]),
            allocatable: capacityValue(name, nodes.allocatable?.[name]),
          }))}
          columns={[
            {
              title: '资源',
              dataIndex: 'name',
              render: (name: string) => resourceLabels[name] ?? name,
            },
            { title: '总容量', dataIndex: 'capacity' },
            { title: '可分配量', dataIndex: 'allocatable' },
          ]}
        />
      )}
    </section>
  );
}
function CapacityCard({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="cluster-capacity-card">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </div>
  );
}
