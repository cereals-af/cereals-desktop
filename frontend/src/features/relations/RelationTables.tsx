import { Alert, Button, Space, Table, Tag } from 'antd';
import type { RelatedNode, RelationSection, ResourceEvent } from '../../api/types';

export function RelationTables({
  section,
  onInspect,
  onYAML,
  onPod,
}: {
  onPod?: (node: RelatedNode) => void;
  section: RelationSection;
  onInspect: (node: RelatedNode) => void;
  onYAML: (node: RelatedNode) => void;
}) {
  const incomplete = Boolean(section.error || section.truncated);
  return (
    <section className="relation-section">
      <h3>{section.title}</h3>
      <p className="muted relation-note">{section.note}</p>
      {section.error && (
        <Alert type="warning" showIcon title={`${section.title} 读取失败：${section.error}`} />
      )}
      {section.truncated && (
        <Alert
          type="warning"
          showIcon
          title="结果不完整：已达到扫描上限或上游关联未完整读取。可到资源列表缩小范围查询。"
        />
      )}
      {section.key === 'events' ? (
        <Table<ResourceEvent>
          size="small"
          rowKey="uid"
          dataSource={section.events ?? []}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
          locale={{
            emptyText: incomplete ? '当前没有可展示的事件，查询未完成' : '当前资源暂无可查询事件',
          }}
          columns={[
            {
              title: '类型 / 原因',
              key: 'type',
              width: 210,
              render: (_, event) => (
                <>
                  <Tag color={event.type === 'Warning' ? 'warning' : 'default'}>{event.type}</Tag>
                  <div>{event.reason}</div>
                </>
              ),
            },
            { title: '消息', dataIndex: 'message' },
            { title: '次数', dataIndex: 'count', width: 65 },
            {
              title: '最近发生',
              dataIndex: 'time',
              width: 180,
              render: (value: string) => (value ? new Date(value).toLocaleString() : '—'),
            },
          ]}
        />
      ) : (
        <Table<RelatedNode>
          size="small"
          rowKey="uid"
          dataSource={section.items ?? []}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
          locale={{ emptyText: incomplete ? '当前没有可展示的资源，查询未完成' : '未发现关联资源' }}
          columns={[
            {
              title: '名称',
              key: 'name',
              render: (_, node) => (
                <Button type="link" className="resource-link" onClick={() => onInspect(node)}>
                  {node.target.name}
                </Button>
              ),
            },
            {
              title: '状态 / 修订',
              key: 'status',
              render: (_, node) => (
                <>
                  {node.status || '—'}
                  {node.revision && <div className="muted">修订 {node.revision}</div>}
                </>
              ),
            },
            {
              title: '节点 / 端点',
              dataIndex: 'detail',
              render: (value: string) => <span className="relation-detail">{value || '—'}</span>,
            },
            { title: '关联依据', dataIndex: 'relation', width: 130 },
            {
              title: '操作',
              key: 'actions',
              width: onPod ? 210 : 80,
              render: (_, node) => (
                <Space>
                  {onPod && node.target.group === '' && node.target.resource === 'pods' && (
                    <>
                      <Button aria-label="日志" size="small" onClick={() => onPod(node)}>
                        日志
                      </Button>
                      <Button aria-label="终端" size="small" onClick={() => onPod(node)}>
                        终端
                      </Button>
                    </>
                  )}
                  <Button size="small" onClick={() => onYAML(node)}>
                    YAML
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      )}
    </section>
  );
}
