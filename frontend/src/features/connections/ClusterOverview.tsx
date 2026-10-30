import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Descriptions, Drawer, Input, Skeleton, Table, Tag } from 'antd';
import { ArrowRight, RefreshCw, Search } from 'lucide-react';
import { useAPI } from '../../api/context';
import type { ClusterOverview as Snapshot, Profile } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { typeKey, typeLabel } from '../resources/catalog';
import { ClusterCapacity } from './ClusterCapacity';

export function ClusterOverview({
  profile,
  onClose,
  onBrowse,
}: {
  profile: Profile;
  onClose: () => void;
  onBrowse: () => void;
}) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', profile.id, 'overview'],
    queryFn: ({ signal }) => api.clusterOverview(profile.id, signal),
    retry: false,
    gcTime: 0,
    refetchOnWindowFocus: false,
  });
  return (
    <Drawer
      open
      destroyOnHidden
      size={1080}
      className="manifest-drawer cluster-overview"
      onClose={onClose}
      title={
        <div className="drawer-title">
          <span>集群详情 · {profile.context}</span>
          <small>{profile.server}</small>
        </div>
      }
    >
      <div className="cluster-overview-toolbar">
        <span className="muted">
          {query.data
            ? `更新于 ${new Date(query.data.updatedAt).toLocaleTimeString('zh-CN')}`
            : '正在读取集群概况'}
        </span>
        <div className="cluster-overview-actions">
          <Button
            className="form-item-action"
            icon={<RefreshCw size={14} />}
            loading={query.isFetching}
            onClick={() => void query.refetch()}
          >
            刷新详情
          </Button>
          <Button
            className="form-item-action"
            type="primary"
            icon={<ArrowRight size={14} />}
            onClick={onBrowse}
          >
            浏览资源
          </Button>
        </div>
      </div>
      <Descriptions
        size="small"
        column={2}
        bordered
        items={[
          { key: 'cluster', label: '集群名称', children: profile.cluster },
          { key: 'version', label: 'Kubernetes 版本', children: profile.version || '—' },
          { key: 'context', label: 'Context', children: profile.context },
          { key: 'namespace', label: '默认命名空间', children: profile.namespace || 'default' },
        ]}
      />
      {query.isError && (
        <Alert
          showIcon
          type="error"
          title={errorMessage(query.error)}
          description="请刷新重试；已显示的数据可能不是最新状态。"
        />
      )}
      {query.isPending && <Skeleton active />}
      {query.data && <OverviewDetails snapshot={query.data} />}
    </Drawer>
  );
}

function OverviewDetails({ snapshot }: { snapshot: Snapshot }) {
  return (
    <>
      {(snapshot.warnings ?? []).map((warning) => (
        <Alert key={warning} showIcon type="warning" title={warning} />
      ))}
      <ClusterCapacity snapshot={snapshot} />
      <ResourceCounts snapshot={snapshot} />
    </>
  );
}

function ResourceCounts({ snapshot }: { snapshot: Snapshot }) {
  const [search, setSearch] = useState('');
  const failed = (snapshot.resources ?? []).filter((item) => item.count === null).length;
  const needle = search.trim().toLocaleLowerCase();
  const rows = (snapshot.resources ?? []).filter(({ type }) =>
    `${typeLabel(type)} ${type.group} ${type.resource}`.toLocaleLowerCase().includes(needle),
  );
  return (
    <section className="cluster-overview-section">
      <div className="cluster-overview-section-heading">
        <div>
          <h3>
            资源数量 <span className="count-badge">{snapshot.resources?.length ?? 0} 类</span>
          </h3>
          <p className="muted">统计全部命名空间；同一资源按发现的首选 API 版本计数。</p>
        </div>
        <Input
          allowClear
          prefix={<Search size={14} />}
          className="cluster-count-search"
          aria-label="搜索资源统计"
          placeholder="搜索资源类型"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      {failed > 0 && (
        <Alert
          showIcon
          type="warning"
          title={`${failed} 类资源无法统计，其他结果仍可查看`}
          description="权限不足或 API 不可用的项目显示原因，不计为 0。"
        />
      )}
      <Table
        size="small"
        rowKey={(item) => typeKey(item.type)}
        dataSource={rows}
        pagination={{ pageSize: 10, showSizeChanger: false, hideOnSinglePage: true }}
        locale={{ emptyText: needle ? '没有匹配的资源类型' : '未获取到资源类型' }}
        columns={[
          {
            title: '资源类型',
            key: 'type',
            render: (_, item) => (
              <div className="cluster-resource-type">
                <strong>{typeLabel(item.type)}</strong>
                <small>
                  {item.type.group || 'core'} / {item.type.version}
                </small>
              </div>
            ),
          },
          {
            title: '作用域',
            key: 'scope',
            width: 140,
            render: (_, item) => (item.type.namespaced ? '全部命名空间' : '集群'),
          },
          {
            title: '数量',
            key: 'count',
            width: 100,
            align: 'right',
            render: (_, item) => (item.count === null ? '—' : item.count.toLocaleString('zh-CN')),
          },
          {
            title: '统计状态',
            key: 'status',
            width: 260,
            render: (_, item) =>
              item.count === null ? (
                <div className="cluster-count-error">
                  <Tag color="warning">无法统计</Tag>
                  <small>{item.error}</small>
                </div>
              ) : (
                <Tag color="success">已统计</Tag>
              ),
          },
        ]}
      />
    </section>
  );
}
