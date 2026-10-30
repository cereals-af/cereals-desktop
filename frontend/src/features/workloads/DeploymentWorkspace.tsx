import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, ConfigProvider, Drawer, Space, Spin, Tabs } from 'antd';
import { ContainerDetails } from './ContainerDetails';
import { WorkloadPods } from './WorkloadPods';
import { RelationsPanel } from '../relations/RelationsPanel';
import { useAPI } from '../../api/context';
import type { Baseline, DeploymentRevision, Profile, Ref, WorkloadSnapshot } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { MutationReview } from '../resources/MutationReview';
import { DeploymentHistory } from './DeploymentHistory';
import { RollbackEditor } from './RollbackEditor';
import { ScaleWorkspace } from './ScaleWorkspace';
import { WorkloadAccess } from './WorkloadAccess';
import { DeploymentFields } from './DeploymentFields';
import { actionLabels } from './actions';
import type { WorkloadAction } from './actions';
import { DeploymentStatus } from './DeploymentStatus';
import { emptyDeployment } from './defaults';
import { useWorkloadDraft } from './useWorkloadDraft';

interface Props {
  target: Ref;
  initialTab?: string;
  expectedUID?: string;
  profile: Profile;
  canPatch: boolean;
  onClose: () => void;
  onYAML: (target: Ref) => void;
  onSuccess: () => void;
}
type View =
  | { kind: 'detail' }
  | { kind: 'form'; action: WorkloadAction; snapshot: WorkloadSnapshot }
  | { kind: 'rollback'; revision: DeploymentRevision; baseline: Baseline };

export function DeploymentWorkspace(props: Props) {
  const api = useAPI();
  const [tab, setTab] = useState(props.initialTab ?? 'status');
  const [scaleOpen, setScaleOpen] = useState(false);
  const [target, setTarget] = useState(props.target);
  const [detailRevision, setDetailRevision] = useState(0);
  const [view, setView] = useState<View>(() =>
    props.target.name
      ? { kind: 'detail' }
      : {
          kind: 'form',
          action: 'create',
          snapshot: {
            form: emptyDeployment(props.target.namespace || props.profile.namespace),
            baseline: { uid: '', resourceVersion: '' },
            status: {
              phase: '',
              strategy: '',
              paused: false,
              observed: false,
              replicas: 0,
              updated: 0,
              ready: 0,
              available: 0,
              conditions: [],
            },
          },
        },
  );
  const query = useQuery({
    queryKey: [
      'cluster',
      target.clusterID,
      'deployment',
      target,
      detailRevision,
      props.expectedUID,
    ],
    queryFn: async ({ signal }) => {
      const snapshot = await api.deployment(target, signal);
      if (props.expectedUID && snapshot.baseline.uid !== props.expectedUID)
        throw new Error('Deployment UID 已变化，请返回所属资源重新选择');
      return snapshot;
    },
    enabled: view.kind === 'detail' && Boolean(target.name),
    refetchInterval: 5000,
    gcTime: 0,
    retry: false,
  });
  const showStatus = (next: Ref) => {
    setTab('status');
    setTarget(next);
    setDetailRevision((revision) => revision + 1);
    setView({ kind: 'detail' });
  };
  return (
    <Drawer
      open
      size="min(1080px, 100vw)"
      destroyOnHidden
      onClose={props.onClose}
      className="manifest-drawer"
      title={
        <div className="drawer-title">
          <span>
            {view.kind === 'form'
              ? actionLabels[view.action]
              : view.kind === 'rollback'
                ? `回滚 Deployment · ${target.name}`
                : `Deployment · ${target.name}`}
          </span>
          <small>
            {props.profile.context} /{' '}
            {view.kind === 'form' && view.action === 'create'
              ? '新建资源 · 选择命名空间'
              : target.namespace}{' '}
            · {props.profile.server}
          </small>
        </div>
      }
    >
      {view.kind === 'rollback' ? (
        <RollbackEditor
          target={target}
          revision={view.revision}
          baseline={view.baseline}
          onSuccess={props.onSuccess}
          onStatus={showStatus}
          onBack={() => setView({ kind: 'detail' })}
        />
      ) : view.kind === 'form' ? (
        <DeploymentEditor
          key={view.action}
          target={target}
          snapshot={view.snapshot}
          action={view.action}
          onSuccess={props.onSuccess}
          onStatus={showStatus}
          onBack={() => setView({ kind: 'detail' })}
        />
      ) : (
        <>
          <Space wrap className="mutation-toolbar">
            <Button onClick={() => props.onYAML(target)}>查看 / 编辑 YAML</Button>
            <Button onClick={() => setScaleOpen(true)}>扩缩容</Button>
            {props.canPatch &&
              (['update', 'restart', query.data?.status.paused ? 'resume' : 'pause'] as const).map(
                (action) => (
                  <Button
                    key={action}
                    type={action === 'update' ? 'primary' : 'default'}
                    disabled={
                      !query.data ||
                      query.isFetching ||
                      query.isError ||
                      (action === 'restart' && query.data.status.paused)
                    }
                    onClick={() => {
                      if (query.data) setView({ kind: 'form', action, snapshot: query.data });
                    }}
                  >
                    {actionLabels[action]}
                  </Button>
                ),
              )}
            <Button loading={query.isFetching} onClick={() => void query.refetch()}>
              刷新状态
            </Button>
          </Space>
          <WorkloadAccess target={target} />
          {query.isError && (
            <Alert
              showIcon
              type="error"
              title={errorMessage(query.error)}
              description="下面的上次快照可能已过期。"
            />
          )}
          {query.isPending && <Spin />}
          <Tabs
            activeKey={tab}
            onChange={setTab}
            items={[
              {
                key: 'status',
                label: '发布状态',
                children: query.data && <DeploymentStatus snapshot={query.data} />,
              },
              {
                key: 'containers',
                label: '容器配置',
                children: query.data && (
                  <ContainerDetails
                    containers={query.data.form.containers}
                    podOptions={query.data.form.podOptions}
                  />
                ),
              },
              {
                key: 'pods',
                label: '容器组',
                children: <WorkloadPods target={target} contextName={props.profile.context} />,
              },
              {
                key: 'history',
                label: '发布历史',
                children: (
                  <DeploymentHistory
                    target={target}
                    onRollback={(revision, baseline) =>
                      setView({ kind: 'rollback', revision, baseline })
                    }
                  />
                ),
              },
              {
                key: 'relations',
                label: '关联资源与事件',
                children: (
                  <RelationsPanel
                    key={JSON.stringify(target)}
                    target={target}
                    contextName={props.profile.context}
                  />
                ),
              },
            ]}
          />
        </>
      )}
      {scaleOpen && (
        <ScaleWorkspace
          target={target}
          profile={props.profile}
          onClose={() => setScaleOpen(false)}
          onSuccess={props.onSuccess}
          onStatus={(next) => {
            setScaleOpen(false);
            showStatus(next);
          }}
        />
      )}
    </Drawer>
  );
}

function DeploymentEditor({
  target,
  snapshot,
  action,
  onSuccess,
  onStatus,
  onBack,
}: {
  target: Ref;
  snapshot: WorkloadSnapshot;
  action: WorkloadAction;
  onSuccess: () => void;
  onStatus: (target: Ref) => void;
  onBack: () => void;
}) {
  const edit = useWorkloadDraft(target, snapshot, action, onSuccess);
  const busy = edit.phase.kind === 'previewing' || edit.phase.kind === 'committing';
  if (edit.phase.kind === 'done') {
    const resultTarget = edit.phase.target;
    return (
      <Alert
        type="success"
        showIcon
        title="请求已提交，等待控制器处理"
        description="请查看发布状态，确认实际副本与可用情况。"
        action={<Button onClick={() => onStatus(resultTarget)}>查看发布状态</Button>}
      />
    );
  }
  return (
    <div className="manifest-session">
      {edit.error && <Alert type="error" showIcon title={edit.error} />}
      {edit.notice && <Alert type="warning" showIcon title={edit.notice} />}
      {edit.phase.kind === 'review' || edit.phase.kind === 'committing' ? (
        <>
          <MutationReview preview={edit.phase.preview} />
          <div className="mutation-actions">
            <Button disabled={busy} onClick={edit.back}>
              返回修改
            </Button>
            <Button
              type="primary"
              aria-label="确认提交到集群"
              loading={busy}
              onClick={() => void edit.commit()}
            >
              确认提交到集群
            </Button>
          </div>
        </>
      ) : (
        <>
          <ConfigProvider componentDisabled={busy}>
            <fieldset disabled={busy} className="workload-fieldset">
              <DeploymentFields
                clusterID={target.clusterID}
                action={action}
                value={edit.form}
                onChange={edit.setForm}
              />
            </fieldset>
          </ConfigProvider>
          <div className="mutation-actions">
            {action !== 'create' && (
              <>
                <Button disabled={busy} onClick={() => void edit.reload()}>
                  重新读取基线（保留草稿）
                </Button>
                <Button disabled={busy} onClick={onBack}>
                  放弃草稿，返回详情
                </Button>
              </>
            )}
            <Button type="primary" loading={busy} onClick={() => void edit.preview()}>
              服务端预检并查看差异
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
