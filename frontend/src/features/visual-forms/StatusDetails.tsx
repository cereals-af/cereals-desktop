import { Descriptions, Table } from 'antd';
import type { CustomValue } from '../../api/types';
import { StatusBadge } from '../resources/StatusBadge';
import { property } from '../custom-resources/values';

const labels: Record<string, string> = {
  active: '活动任务',
  succeeded: '成功数',
  failed: '失败数',
  ready: '就绪数',
  startTime: '开始时间',
  completionTime: '完成时间',
  lastScheduleTime: '上次调度',
  lastSuccessfulTime: '上次成功',
  completedIndexes: '完成索引',
  failedIndexes: '失败索引',
  terminating: '终止中',
};
export function StatusDetails({
  value,
  state,
  detail,
}: {
  value: CustomValue;
  state?: string;
  detail?: string;
}) {
  const values = Object.entries(value.properties ?? {}).filter(
    (entry): entry is [string, CustomValue] =>
      Boolean(entry[1] && !['object', 'array', 'null'].includes(entry[1].type)),
  );
  const conditions = property(value, 'conditions')?.items ?? [];
  const active = property(value, 'active')?.items ?? [];
  const addresses = property(property(value, 'loadBalancer') ?? value, 'ingress')?.items ?? [];
  return (
    <div className="container-section">
      {state && (
        <div className="workload-state-heading">
          <StatusBadge status={state} />
          <span className="muted">{detail}</span>
        </div>
      )}
      {values.length > 0 && (
        <Descriptions
          size="small"
          bordered
          column={2}
          items={values.map(([key, v]) => ({ key, label: labels[key] ?? key, children: v.text }))}
        />
      )}
      {active.length > 0 && (
        <p>活动 Job：{active.map((v) => property(v, 'name')?.text).join('、')}</p>
      )}
      {addresses.length > 0 && (
        <p>
          入口地址：
          {addresses
            .map((v) => property(v, 'ip')?.text || property(v, 'hostname')?.text)
            .join('、')}
        </p>
      )}
      {conditions.length > 0 && (
        <Table<CustomValue>
          size="small"
          pagination={false}
          rowKey={(v) => property(v, 'type')?.text ?? ''}
          dataSource={conditions}
          columns={[
            ['type', '条件'],
            ['status', '状态'],
            ['reason', '原因'],
            ['message', '说明'],
          ].map(([key, title]) => ({
            title,
            render: (_, v) => property(v, key ?? '')?.text ?? '—',
          }))}
        />
      )}
    </div>
  );
}
