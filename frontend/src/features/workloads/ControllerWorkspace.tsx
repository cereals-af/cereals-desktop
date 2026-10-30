import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Drawer, Space, Spin, Tabs } from 'antd';
import { useAPI } from '../../api/context';
import type {
  Baseline,
  ControllerSnapshot,
  DeploymentRevision,
  Profile,
  Ref,
} from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { FormSession } from '../resources/FormSession';
import { RelationsPanel } from '../relations/RelationsPanel';
import { controllerKind, emptyController, strategyNote } from './controllers';
import { ControllerFields } from './ControllerFields';
import { ControllerDetails } from './ControllerDetails';
import { ContainerDetails } from './ContainerDetails';
import { ScaleWorkspace } from './ScaleWorkspace';
import { WorkloadPods } from './WorkloadPods';
import { DeploymentHistory } from './DeploymentHistory';
import { RollbackEditor } from './RollbackEditor';
import { StatefulStorage } from './StatefulStorage';
import { DeploymentWorkspace } from './DeploymentWorkspace';

interface Props {
  target: Ref;
  profile: Profile;
  canPatch: boolean;
  initialTab?: string;
  onClose: () => void;
  onSuccess: () => void;
  onYAML: (target: Ref) => void;
}
type View =
  | { kind: 'detail' }
  | { kind: 'edit'; action: 'create' | 'update' | 'restart'; snapshot: ControllerSnapshot | null }
  | { kind: 'rollback'; revision: DeploymentRevision; baseline: Baseline };
export function ControllerWorkspace(props: Props) {
  const api = useAPI();
  const cache = useQueryClient();
  const [target, setTarget] = useState(props.target);
  const [view, setView] = useState<View>(
    target.name ? { kind: 'detail' } : { kind: 'edit', action: 'create', snapshot: null },
  );
  const [tab, setTab] = useState(props.initialTab ?? 'status');
  const [scale, setScale] = useState(false);
  const [owner, setOwner] = useState<{ target: Ref; uid: string } | null>(null);
  const [ownerError, setOwnerError] = useState('');
  const kind = controllerKind(target);
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'controller', target],
    queryFn: ({ signal }) => api.controller(target, signal),
    enabled: Boolean(target.name),
    retry: false,
    gcTime: 0,
    refetchInterval: view.kind === 'detail' ? 5000 : false,
  });
  const done = (next: Ref) => {
    setTab('status');
    void cache.invalidateQueries({
      queryKey: ['cluster', target.clusterID, 'controller-history', target],
    });
    setTarget(next);
    setView({ kind: 'detail' });
    if (next.name === target.name) void query.refetch();
  };
  const openOwner = async () => {
    const parent = query.data?.owner;
    if (!parent) return;
    try {
      const snapshot = await api.deployment(parent);
      if (snapshot.baseline.uid !== query.data?.ownerUID)
        throw new Error('所属 Deployment UID 已变化，请刷新工作负载后重新选择');
      setOwner({ target: parent, uid: snapshot.baseline.uid });
      setOwnerError('');
    } catch (error) {
      setOwnerError(errorMessage(error));
    }
  };
  return (
    <Drawer
      open
      size="min(1080px, 100vw)"
      destroyOnHidden
      className="manifest-drawer"
      onClose={props.onClose}
      title={
        <div className="drawer-title">
          <span>
            {kind} · {target.name || '创建资源'}
          </span>
          <small>
            {props.profile.context} /{' '}
            {view.kind === 'edit' && view.action === 'create'
              ? '新建资源 · 选择命名空间'
              : target.namespace}{' '}
            · {props.profile.server}
          </small>
        </div>
      }
    >
      <div hidden={view.kind !== 'detail'}>
        <Space wrap className="mutation-toolbar">
          <Button onClick={() => props.onYAML(target)}>查看 / 编辑 YAML</Button>
          {target.resource !== 'daemonsets' && (
            <Button onClick={() => setScale(true)}>扩缩容</Button>
          )}
          {props.canPatch &&
            (['update', ...(target.resource === 'replicasets' ? [] : ['restart'])] as const).map(
              (action) => (
                <Button
                  key={action}
                  type={action === 'update' ? 'primary' : 'default'}
                  disabled={!query.data || query.isFetching || query.isError}
                  onClick={() => {
                    if (query.data && (action === 'update' || action === 'restart'))
                      setView({ kind: 'edit', action, snapshot: query.data });
                  }}
                >
                  {action === 'update' ? '更新配置' : '重启工作负载'}
                </Button>
              ),
            )}
          <Button loading={query.isFetching} onClick={() => void query.refetch()}>
            刷新状态
          </Button>
        </Space>
        {query.data?.owner && (
          <Alert
            showIcon
            type="info"
            title={`由 Deployment ${query.data.owner.name} 管理`}
            description="优先在所属 Deployment 修改模板或副本数，控制器可能覆盖此 ReplicaSet 的直接修改。"
            action={<Button onClick={() => void openOwner()}>进入所属 Deployment</Button>}
          />
        )}
        {ownerError && <Alert type="error" title={ownerError} />}
        {query.isError && (
          <Alert
            showIcon
            type="error"
            title={errorMessage(query.error)}
            description="上次快照可能已过期。"
          />
        )}
        {query.isPending && <Spin />}
        {target.name && (
          <Tabs
            activeKey={tab}
            onChange={setTab}
            items={[
              {
                key: 'status',
                label: '工作负载状态',
                children: query.data && <ControllerDetails target={target} snapshot={query.data} />,
              },
              {
                key: 'containers',
                label: '容器配置',
                children: query.data && (
                  <ContainerDetails
                    containers={query.data.form.workload.containers}
                    podOptions={query.data.form.workload.podOptions}
                  />
                ),
              },
              {
                key: 'pods',
                label: '容器组',
                children: <WorkloadPods target={target} contextName={props.profile.context} />,
              },
              ...(target.resource === 'statefulsets'
                ? [
                    {
                      key: 'storage',
                      label: '存储与 PVC',
                      children: query.data && (
                        <StatefulStorage
                          target={target}
                          profile={props.profile}
                          snapshot={query.data}
                          onSuccess={props.onSuccess}
                        />
                      ),
                    },
                  ]
                : []),
              ...(target.resource !== 'replicasets'
                ? [
                    {
                      key: 'history',
                      label: '发布历史',
                      children: (
                        <DeploymentHistory
                          controller
                          target={target}
                          onRollback={(revision, baseline) =>
                            setView({ kind: 'rollback', revision, baseline })
                          }
                        />
                      ),
                    },
                  ]
                : []),
              {
                key: 'relations',
                label: '关联资源与事件',
                children: <RelationsPanel target={target} contextName={props.profile.context} />,
              },
            ]}
          />
        )}
      </div>
      {view.kind === 'edit' && (
        <FormSession
          key={view.action}
          initial={
            view.snapshot?.form ?? emptyController(target.namespace || props.profile.namespace)
          }
          baseline={view.snapshot?.baseline ?? { uid: '', resourceVersion: '' }}
          creating={view.action === 'create'}
          prepare={(form, baseline, signal) =>
            api.previewController(
              {
                target:
                  view.action === 'create'
                    ? { ...target, namespace: form.workload.namespace }
                    : target,
                action: view.action,
                form,
                baseline,
              },
              signal,
            )
          }
          readBaseline={async (signal) => (await api.controller(target, signal)).baseline}
          onSuccess={props.onSuccess}
          onDone={done}
          onBack={() => setView({ kind: 'detail' })}
          doneLabel="查看工作负载状态"
        >
          {(form, setForm) => (
            <ControllerFields
              clusterID={target.clusterID}
              resource={target.resource}
              kind={kind}
              action={view.action}
              value={form}
              onChange={setForm}
            />
          )}
        </FormSession>
      )}
      {view.kind === 'rollback' && (
        <>
          {query.data && (
            <Alert showIcon type="warning" title={strategyNote(target.resource, query.data.form)} />
          )}
          <RollbackEditor
            controller
            target={target}
            revision={view.revision}
            baseline={view.baseline}
            onSuccess={props.onSuccess}
            onStatus={done}
            onBack={() => setView({ kind: 'detail' })}
          />
        </>
      )}
      {scale && (
        <ScaleWorkspace
          target={target}
          profile={props.profile}
          onClose={() => setScale(false)}
          onSuccess={props.onSuccess}
          onStatus={() => {
            setScale(false);
            void query.refetch();
          }}
        />
      )}
      {owner && (
        <DeploymentWorkspace
          target={owner.target}
          expectedUID={owner.uid}
          profile={props.profile}
          canPatch={props.canPatch}
          onClose={() => setOwner(null)}
          onSuccess={props.onSuccess}
          onYAML={props.onYAML}
        />
      )}
    </Drawer>
  );
}
