import { Alert, Descriptions, Table } from 'antd';
import type { ControllerSnapshot, Ref } from '../../api/types';
import { StatusBadge } from '../resources/StatusBadge';
import { strategyNote } from './controllers';

export function ControllerDetails({
  snapshot,
  target,
}: {
  snapshot: ControllerSnapshot;
  target: Ref;
}) {
  const { form, status } = snapshot;
  const daemon = target.resource === 'daemonsets';
  return (
    <div className="deployment-status">
      <div className="rollout-heading">
        <h2>工作负载状态</h2>
        <div className="workload-state-heading">
          <StatusBadge status={status.state || (status.ready > 0 ? 'Ready' : 'Processing')} />
          <span className="muted">{status.phase}</span>
        </div>
      </div>
      {status.detail && <p className="muted">{status.detail}</p>}
      <div className="replica-grid">
        {[
          ['期望', status.desired],
          ['当前', status.current],
          ['就绪', status.ready],
          ...(target.resource !== 'replicasets' ? [['已更新', status.updated]] : []),
          ['可用', status.available],
          ['不可用', status.unavailable],
          ...(daemon ? [['错误调度', status.misscheduled]] : []),
        ].map(([label, value]) => (
          <div key={label}>
            <span>
              {label}
              {daemon ? '节点 / Pod' : '副本'}
            </span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      <Alert showIcon type="info" title={strategyNote(target.resource, form)} />
      <Descriptions
        size="small"
        column={2}
        items={[
          {
            key: 'observed',
            label: '控制器已处理当前版本',
            children: status.observed ? '是' : '否',
          },
          { key: 'version', label: '资源版本', children: snapshot.baseline.resourceVersion },
          ...(target.resource === 'statefulsets'
            ? [
                { key: 'service', label: '治理 Service', children: form.serviceName },
                { key: 'order', label: 'Pod 管理顺序', children: form.podManagementPolicy },
                {
                  key: 'current',
                  label: 'currentRevision',
                  children: status.currentRevision || '未上报',
                },
                {
                  key: 'update',
                  label: 'updateRevision',
                  children: status.updateRevision || '未上报',
                },
                { key: 'scale', label: '缩容 PVC 策略', children: form.retainScaled },
                { key: 'delete', label: '删除 PVC 策略', children: form.retainDeleted },
              ]
            : []),
        ]}
      />
      <p className="muted">
        每 5 秒读取状态。提交成功仅表示 API Server 接受请求；DaemonSet
        的期望数量由匹配节点决定，不提供手动副本数。
      </p>
      <h3>调度约束</h3>
      <pre className="config-preview">{status.scheduling}</pre>
      <h3>控制器条件</h3>
      <Table
        size="small"
        rowKey="type"
        pagination={false}
        dataSource={status.conditions ?? []}
        locale={{ emptyText: '控制器尚未上报条件' }}
        columns={[
          { title: '条件', dataIndex: 'type' },
          { title: '状态', dataIndex: 'status' },
          { title: '原因', dataIndex: 'reason' },
          { title: '消息', dataIndex: 'message' },
        ]}
      />
    </div>
  );
}
