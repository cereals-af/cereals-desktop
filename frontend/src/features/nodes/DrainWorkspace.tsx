import { useEffect, useRef, useState } from 'react';
import {
  Alert,
  Button,
  Checkbox,
  Drawer,
  Input,
  InputNumber,
  Progress,
  Space,
  Table,
  Tag,
} from 'antd';
import type { NodeSnapshot, PodProgress, Profile, Ref } from '../../api/types';
import { MutationReview } from '../resources/MutationReview';
import { useDrain } from './useDrain';

export function DrainWorkspace({
  target,
  snapshot,
  profile,
  onClose,
  onSuccess,
}: {
  target: Ref;
  snapshot: NodeSnapshot;
  profile: Profile;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const drain = useDrain(target, snapshot, onSuccess);
  const [name, setName] = useState('');
  const top = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (drain.phase.kind === 'running') top.current?.scrollIntoView({ block: 'start' });
  }, [drain.phase.kind]);
  const busy = drain.phase.kind === 'previewing' || drain.phase.kind === 'starting';
  const review =
    drain.phase.kind === 'review' || drain.phase.kind === 'starting' ? drain.phase.plan : null;
  const progress = drain.phase.kind === 'running' ? drain.phase.progress : null;
  return (
    <Drawer
      open
      size={1160}
      className="manifest-drawer"
      destroyOnHidden
      onClose={onClose}
      title={
        <div className="drawer-title">
          <span>排空节点 · {target.name}</span>
          <small>
            {profile.context} · {profile.server}
          </small>
        </div>
      }
    >
      <div ref={top} />
      <Alert
        type="warning"
        showIcon
        title="排空会先封锁节点，再请求驱逐已确认的 Pod"
        description="遵守 PDB；等待原 Pod 删除，不保证替代 Pod 已就绪。取消或关闭窗口会停止后续操作，已接受的驱逐不会回滚，节点不会自动解封。"
      />
      {drain.error && (
        <Alert
          type="error"
          showIcon
          title={drain.error}
          action={
            drain.phase.kind === 'running' ? (
              <Button onClick={() => void drain.retry()}>重新读取进度</Button>
            ) : undefined
          }
        />
      )}
      {drain.phase.kind === 'editing' || drain.phase.kind === 'previewing' ? (
        <div className="drain-options">
          <Checkbox
            disabled={busy}
            checked={drain.options.ignoreDaemonSets}
            onChange={(event) =>
              drain.setOptions({ ...drain.options, ignoreDaemonSets: event.target.checked })
            }
          >
            保留 DaemonSet Pod 并继续排空
          </Checkbox>
          <Checkbox
            disabled={busy}
            checked={drain.options.allowUnmanaged}
            onChange={(event) =>
              drain.setOptions({ ...drain.options, allowUnmanaged: event.target.checked })
            }
          >
            允许驱逐无控制器引用的 Pod（可能不会重建）
          </Checkbox>
          <Checkbox
            disabled={busy}
            checked={drain.options.allowLocalData}
            onChange={(event) =>
              drain.setOptions({ ...drain.options, allowLocalData: event.target.checked })
            }
          >
            接受 emptyDir / hostPath 本地数据丢失或不可迁移的影响
          </Checkbox>
          <label>
            任务超时（秒）{' '}
            <InputNumber
              aria-label="排空超时秒数"
              min={30}
              max={3600}
              disabled={busy}
              value={drain.options.timeoutSeconds}
              onChange={(value) =>
                drain.setOptions({ ...drain.options, timeoutSeconds: value ?? 600 })
              }
            />
          </label>
          <p className="muted">
            静态 Pod 保留；读取完整节点 Pod 清单，最多 500
            个。预检只验证当时的权限和准入，执行时会重新校验。
          </p>
          <Button type="primary" loading={busy} onClick={() => void drain.preview()}>
            检查排空影响范围
          </Button>
        </div>
      ) : null}
      {review && (
        <>
          <h3>待处理 Pod</h3>
          <DrainPods rows={review.pods ?? []} />
          {!review.token && (
            <Alert type="error" title="存在阻塞项，请返回调整策略或处理权限问题后重新预检" />
          )}
          <h3>节点调度变更</h3>
          <MutationReview preview={review.cordon} />
          <Space className="mutation-toolbar">
            <Button disabled={busy} onClick={drain.back}>
              返回调整策略
            </Button>
            <Input
              aria-label="确认排空节点名称"
              placeholder={`输入 ${target.name} 确认`}
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={busy}
            />
            <Button
              danger
              type="primary"
              loading={busy}
              disabled={!review.token || name !== target.name}
              onClick={() => void drain.start(name)}
            >
              确认封锁并排空
            </Button>
          </Space>
        </>
      )}
      {drain.phase.kind === 'running' && (
        <div className="drain-progress">
          <h3>{progress?.done ? '排空任务结果' : '排空任务执行中'}</h3>
          {progress && (
            <>
              <Alert
                showIcon
                type={
                  progress.state === 'completed' ? 'success' : progress.done ? 'warning' : 'info'
                }
                title={progress.message}
              />
              <Progress
                percent={
                  progress.state === 'completed'
                    ? 100
                    : Math.round(
                        (100 *
                          (progress.pods ?? []).filter(
                            (row) => row.state === 'done' || row.state === 'skipped',
                          ).length) /
                          Math.max(progress.pods?.length ?? 0, 1),
                      )
                }
                status={
                  progress.state === 'completed'
                    ? 'success'
                    : progress.done
                      ? 'exception'
                      : 'active'
                }
              />
              <DrainPods rows={progress.pods ?? []} />
            </>
          )}
          <Space className="mutation-toolbar">
            <Button danger disabled={progress?.done} onClick={() => void drain.cancel()}>
              停止后续操作
            </Button>
            <Button onClick={onClose}>{progress?.done ? '返回节点详情' : '关闭并停止任务'}</Button>
          </Space>
        </div>
      )}
    </Drawer>
  );
}
const labels: Record<string, string> = {
  pending: '等待驱逐',
  blocked: '阻塞',
  skipped: '保留',
  waiting: '等待删除',
  accepted: '驱逐已接受',
  done: '已离开',
  failed: '失败',
};
function DrainPods({ rows }: { rows: PodProgress[] }) {
  return (
    <Table<PodProgress>
      rowKey={(row) => row.pod.uid}
      dataSource={rows}
      pagination={{ pageSize: 20 }}
      size="small"
      scroll={{ x: 850 }}
      columns={[
        {
          title: 'Pod / 命名空间',
          key: 'pod',
          render: (_, row) => (
            <div>
              {row.pod.name}
              <div className="muted">{row.pod.namespace}</div>
            </div>
          ),
        },
        {
          title: '控制器 / 本地数据',
          key: 'owner',
          render: (_, row) => (
            <div>
              {row.pod.owner || '无控制器引用'}
              <div className="muted">{row.pod.localData?.join(', ')}</div>
            </div>
          ),
        },
        {
          title: '处理状态',
          key: 'state',
          render: (_, row) => (
            <Tag
              color={
                row.state === 'done'
                  ? 'green'
                  : row.state === 'failed' || row.state === 'blocked'
                    ? 'red'
                    : 'default'
              }
            >
              {labels[row.state] ?? row.state}
            </Tag>
          ),
        },
        { title: '原因 / 进度', dataIndex: 'message' },
      ]}
    />
  );
}
