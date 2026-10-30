import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Space, Table, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { Baseline, DeploymentRevision, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';

export function DeploymentHistory({
  target,
  onRollback,
  controller = false,
}: {
  controller?: boolean;
  target: Ref;
  onRollback: (revision: DeploymentRevision, baseline: Baseline) => void;
}) {
  const api = useAPI();
  const history = useQuery({
    queryKey: [
      'cluster',
      target.clusterID,
      controller ? 'controller-history' : 'deployment-history',
      target,
    ],
    queryFn: ({ signal }) =>
      controller ? api.controllerHistory(target, signal) : api.deploymentHistory(target, signal),
    retry: false,
    gcTime: 0,
    staleTime: 0,
    refetchOnWindowFocus: false,
  });
  return (
    <div className="deployment-history">
      <div className="relation-toolbar">
        <span>
          已保留的 {controller ? 'ControllerRevision' : 'ReplicaSet'} 修订 · 当前记录{' '}
          {history.data?.currentRevision || '未提供'}
        </span>
        <Button loading={history.isFetching} onClick={() => void history.refetch()}>
          刷新发布历史
        </Button>
      </div>
      <Alert
        type="info"
        showIcon
        title={`历史版本取决于集群保留的 ${controller ? 'ControllerRevision' : 'ReplicaSet'}`}
        description="回滚恢复选定版本的完整 Pod 模板，副本数、发布策略、PVC 模板和工作负载元数据保持当前值；历史模板中的旧配置也会恢复。提交后仍遵循当前更新策略。"
      />
      {history.isError && <Alert showIcon type="error" title={errorMessage(history.error)} />}
      {history.data?.paused && (
        <Alert showIcon type="warning" title="发布已暂停，请先恢复发布再回滚" />
      )}
      {history.data?.truncated && (
        <Alert type="warning" title="历史查询已截断，仅展示已读取的版本" />
      )}
      {history.data?.warnings?.map((warning) => (
        <Alert key={warning} type="warning" title={warning} />
      ))}
      <Table<DeploymentRevision>
        rowKey={(item) => item.baseline.uid}
        loading={history.isFetching}
        dataSource={history.data?.revisions ?? []}
        pagination={{ pageSize: 10, showSizeChanger: false }}
        scroll={{ x: 800 }}
        locale={{
          emptyText:
            history.isError || history.data?.warnings?.length
              ? '查询未完成，不能判断是否存在历史版本'
              : `尚无可读取的历史 ${controller ? 'ControllerRevision' : 'ReplicaSet'}`,
        }}
        columns={[
          {
            title: '修订',
            dataIndex: 'revision',
            width: 80,
            render: (value: string) => value || '—',
          },
          {
            title: controller ? 'ControllerRevision' : 'ReplicaSet',
            dataIndex: 'name',
            render: (value: string, item) => (
              <div>
                <strong>{value}</strong>
                <div className="muted">
                  {item.createdAt ? new Date(item.createdAt).toLocaleString() : '未知时间'}
                </div>
              </div>
            ),
          },
          {
            title: '容器镜像',
            dataIndex: 'images',
            render: (_: unknown, item) => (
              <div className="history-images">
                {item.images?.map((value) => (
                  <span key={value}>{value}</span>
                ))}
              </div>
            ),
          },
          ...(!controller
            ? [
                {
                  title: '副本',
                  width: 90,
                  render: (_: unknown, item: DeploymentRevision) =>
                    `${item.ready} / ${item.replicas}`,
                },
              ]
            : []),
          {
            title: '操作',
            width: 170,
            render: (_: unknown, item) => (
              <Space orientation="vertical">
                {item.current ? (
                  <Tag color="green">当前模板</Tag>
                ) : (
                  <Button
                    size="small"
                    disabled={
                      Boolean(item.unavailable) ||
                      history.data?.paused ||
                      history.isFetching ||
                      history.isError
                    }
                    onClick={() => {
                      if (history.data) onRollback(item, history.data.baseline);
                    }}
                  >
                    回滚到此版本
                  </Button>
                )}
                {item.unavailable && <span className="muted">{item.unavailable}</span>}
              </Space>
            ),
          },
        ]}
      />
    </div>
  );
}
