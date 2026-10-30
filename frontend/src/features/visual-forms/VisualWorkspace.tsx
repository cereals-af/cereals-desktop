import { StatusDetails } from './StatusDetails';
import { useState } from 'react';
import { Alert, Button, Drawer, Space, Spin, Tabs } from 'antd';
import type { Profile, Ref, VisualSnapshot } from '../../api/types';
import { DeleteResource } from '../resources/DeleteResource';
import { VisualEditor, RunJobEditor } from './VisualEditor';
import { RelationsPanel } from '../relations/RelationsPanel';
import { WorkloadPods } from '../workloads/WorkloadPods';
import { ResourceFields } from './ResourceFields';
import { newSnapshot, suspended, toggleSuspend, visualKind } from './models';
import { useSnapshot } from './useSnapshot';

type View =
  | { kind: 'detail' }
  | { kind: 'edit'; snapshot: VisualSnapshot; action?: 'suspend' }
  | { kind: 'run'; snapshot: VisualSnapshot };
export function VisualWorkspace({
  target: initialTarget,
  profile,
  canPatch,
  canDelete,
  onClose,
  onSuccess,
  onYAML,
  initialTab = 'detail',
}: {
  target: Ref;
  profile: Profile;
  canPatch: boolean;
  canDelete: boolean;
  onClose: () => void;
  onSuccess: () => void;
  onYAML: (target: Ref) => void;
  initialTab?: string;
}) {
  const [target, setTarget] = useState(initialTarget);
  const kind = visualKind(target);
  const [view, setView] = useState<View>(() =>
    initialTarget.name || !kind
      ? { kind: 'detail' }
      : { kind: 'edit', snapshot: newSnapshot(kind, initialTarget) },
  );
  const [reveal, setReveal] = useState(false);
  const [revision, setRevision] = useState(0);
  const [deleting, setDeleting] = useState(false);
  const query = useSnapshot(
    target,
    reveal,
    revision,
    view.kind === 'detail' && Boolean(target.name),
  );
  if (!kind) return null;
  const batch = kind === 'Job' || kind === 'CronJob';
  const refresh = () => {
    query.clear();
    setRevision((n) => n + 1);
  };
  const done = (next: Ref) => {
    query.clear();
    setTarget(next);
    setView({ kind: 'detail' });
    setRevision((n) => n + 1);
  };
  const back = () => {
    query.clear();
    setView({ kind: 'detail' });
  };
  const snapshot = query.data;
  return (
    <Drawer
      open
      destroyOnHidden
      size="min(1080px, 100vw)"
      className="manifest-drawer visual-workspace"
      onClose={onClose}
      title={
        <div className="drawer-title">
          <span>
            {target.name ? `${kind} · ${target.name}` : `创建 ${kind}`}
            {view.kind === 'edit' && target.name ? ' · 可视化编辑' : ''}
          </span>
          <small>
            {profile.context} / {target.namespace} · {profile.server}
          </small>
        </div>
      }
    >
      {view.kind === 'run' ? (
        <RunJobEditor
          target={target}
          snapshot={view.snapshot}
          onSuccess={onSuccess}
          onDone={done}
          onBack={back}
        />
      ) : view.kind === 'edit' ? (
        <VisualEditor
          target={target}
          snapshot={view.snapshot}
          kind={kind}
          action={view.action}
          onSuccess={onSuccess}
          onDone={done}
          onBack={back}
        />
      ) : (
        <>
          <Space wrap className="mutation-toolbar">
            <Button onClick={() => onYAML(target)}>查看 / 编辑 YAML</Button>
            {kind === 'Secret' && (
              <Button
                onClick={() => {
                  query.clear();
                  setReveal(!reveal);
                }}
              >
                {reveal ? '隐藏配置' : '解码查看'}
              </Button>
            )}
            {canPatch && (
              <Button
                type="primary"
                disabled={!snapshot || query.loading || (kind === 'Secret' && !reveal)}
                onClick={() => {
                  if (snapshot) setView({ kind: 'edit', snapshot });
                }}
              >
                可视化编辑
              </Button>
            )}
            {batch && canPatch && (
              <Button
                aria-label={snapshot && suspended(snapshot.value) ? '恢复' : '暂停'}
                disabled={!snapshot || query.loading}
                onClick={() => {
                  if (snapshot)
                    setView({
                      kind: 'edit',
                      snapshot: { ...snapshot, value: toggleSuspend(snapshot.value) },
                      action: 'suspend',
                    });
                }}
              >
                {snapshot && suspended(snapshot.value)
                  ? kind === 'CronJob'
                    ? '恢复调度'
                    : '恢复执行'
                  : kind === 'CronJob'
                    ? '暂停调度'
                    : '暂停执行'}
              </Button>
            )}
            {kind === 'CronJob' && (
              <Button
                disabled={!snapshot || query.loading}
                onClick={() => {
                  if (snapshot) setView({ kind: 'run', snapshot });
                }}
              >
                手动运行
              </Button>
            )}
            <Button loading={query.loading} onClick={refresh}>
              刷新详情
            </Button>
            {canDelete && (
              <Button danger onClick={() => setDeleting(true)}>
                删除资源
              </Button>
            )}
          </Space>
          {query.error && <Alert showIcon type="error" title={query.error} />}
          {query.loading && <Spin />}
          {snapshot && (
            <Tabs
              defaultActiveKey={initialTab === 'pods' ? 'pods' : 'detail'}
              items={[
                {
                  key: 'detail',
                  label: '资源详情',
                  children: (
                    <>
                      <StatusDetails
                        value={snapshot.status}
                        state={snapshot.state}
                        detail={snapshot.detail}
                      />
                      <ResourceFields
                        snapshot={snapshot}
                        kind={kind}
                        creating={false}
                        clusterID={target.clusterID}
                        onChange={() => undefined}
                        readOnly
                      />
                    </>
                  ),
                },
                ...(batch
                  ? [
                      {
                        key: 'pods',
                        label: '容器组',
                        forceRender: true,
                        children: <WorkloadPods target={target} contextName={profile.context} />,
                      },
                    ]
                  : []),
                {
                  key: 'relations',
                  label: '关联资源与事件',
                  children: <RelationsPanel target={target} contextName={profile.context} />,
                },
              ]}
            />
          )}
        </>
      )}
      {deleting && (
        <DeleteResource
          target={target}
          profile={profile}
          onClose={() => setDeleting(false)}
          onSuccess={() => {
            onSuccess();
            onClose();
          }}
        />
      )}
    </Drawer>
  );
}
