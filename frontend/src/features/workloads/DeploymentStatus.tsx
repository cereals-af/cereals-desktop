import { Alert, Descriptions, Table } from 'antd';
import { StatusBadge } from '../resources/StatusBadge';
import type { WorkloadSnapshot } from '../../api/types';

export function DeploymentStatus({ snapshot }: { snapshot: WorkloadSnapshot }) {
  const { form, status } = snapshot;
  return (
    <div className="deployment-status">
      <div className="rollout-heading">
        <h2>发布状态</h2>
        <div className="workload-state-heading">
          <StatusBadge status={status.state || (status.ready > 0 ? 'Ready' : 'Processing')} />
          <span className="muted">{status.phase}</span>
        </div>
      </div>
      {status.detail && <p className="muted">{status.detail}</p>}
      <div className="replica-grid">
        {(
          [
            ['期望副本', form.replicas],
            ['已更新', status.updated],
            ['就绪', status.ready],
            ['可用', status.available],
          ] as const
        ).map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      <Descriptions
        size="small"
        column={2}
        items={[
          { key: 'strategy', label: '发布策略', children: status.strategy || 'RollingUpdate' },
          {
            key: 'observed',
            label: '控制器已处理当前版本',
            children: status.observed ? '是' : '否',
          },
          { key: 'replicas', label: '当前副本（含旧版本）', children: status.replicas },
          { key: 'version', label: '资源版本', children: snapshot.baseline.resourceVersion },
        ]}
      />
      <Alert
        type="info"
        showIcon
        title="每 5 秒读取发布状态；提交成功仅表示 API Server 已接受请求。"
      />
      <h3>容器镜像</h3>
      {(form.containers ?? []).map((container) => (
        <div className="container-summary" key={container.name}>
          <strong>{container.name}</strong>
          <code>{container.image}</code>
        </div>
      ))}
      <h3>控制器条件</h3>
      <Table
        rowKey="type"
        size="small"
        pagination={false}
        dataSource={status.conditions ?? []}
        locale={{ emptyText: '控制器尚未上报条件' }}
        columns={[
          { title: '条件', dataIndex: 'type', width: 150 },
          { title: '状态', dataIndex: 'status', width: 75 },
          {
            title: '原因 / 消息',
            key: 'message',
            render: (_, condition) => (
              <div>
                <strong>{condition.reason}</strong>
                <p>{condition.message}</p>
              </div>
            ),
          },
        ]}
      />
    </div>
  );
}
