import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Drawer, Space, Spin, Tabs } from 'antd';
import { useAPI } from '../../api/context';
import type { Profile, Ref, StorageClaim } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { RelationsPanel } from '../relations/RelationsPanel';
import { ClaimCreator, ClaimExpander } from './ClaimEditor';
import { StorageDetails } from './StorageDetails';
import { loadStorage } from './models';

type View = { kind: 'detail' } | { kind: 'create' } | { kind: 'expand'; claim: StorageClaim };

export function StorageWorkspace({
  target: initialTarget,
  profile,
  canPatch,
  onClose,
  onSuccess,
  onYAML,
}: {
  target: Ref;
  profile: Profile;
  canPatch: boolean;
  onClose: () => void;
  onSuccess: () => void;
  onYAML: (target: Ref) => void;
}) {
  const api = useAPI();
  const [target, setTarget] = useState(initialTarget);
  const [view, setView] = useState<View>({ kind: target.name ? 'detail' : 'create' });
  const [revision, setRevision] = useState(0);
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'storage', target, revision],
    queryFn: ({ signal }) => loadStorage(api, target, signal),
    enabled: view.kind === 'detail',
    gcTime: 0,
    retry: false,
  });
  const label =
    target.resource === 'persistentvolumeclaims'
      ? 'PVC'
      : target.resource === 'persistentvolumes'
        ? 'PV'
        : 'StorageClass';
  const done = (ref: Ref) => {
    setTarget(ref);
    setRevision((value) => value + 1);
    setView({ kind: 'detail' });
  };
  const editorProps = {
    target,
    namespace: target.namespace || profile.namespace,
    onSuccess,
    onDone: done,
    onBack: () => setView({ kind: 'detail' }),
  };
  return (
    <Drawer
      open
      destroyOnHidden
      size={1080}
      className="manifest-drawer storage-workspace"
      onClose={onClose}
      title={
        <div className="drawer-title">
          <span>
            {target.name ? `${label} · ${target.name}` : '创建 PVC'}
            {view.kind === 'expand' ? ' · 扩容' : ''}
          </span>
          <small>
            {profile.context} / {target.namespace || '集群作用域'} · {profile.server}
          </small>
        </div>
      }
    >
      {view.kind === 'create' ? (
        <ClaimCreator {...editorProps} />
      ) : view.kind === 'expand' ? (
        <ClaimExpander {...editorProps} claim={view.claim} />
      ) : (
        <>
          <Space wrap className="mutation-toolbar">
            <Button onClick={() => onYAML(target)}>查看 / 编辑 YAML</Button>
            {canPatch && target.resource === 'persistentvolumeclaims' && (
              <Button
                type="primary"
                disabled={
                  query.isFetching ||
                  query.isError ||
                  query.data?.kind !== 'claim' ||
                  query.data.value.phase !== 'Bound' ||
                  query.data.value.deleting
                }
                onClick={() => {
                  if (query.data?.kind === 'claim')
                    setView({ kind: 'expand', claim: query.data.value });
                }}
              >
                申请扩容
              </Button>
            )}
            <Button loading={query.isFetching} onClick={() => void query.refetch()}>
              刷新存储详情
            </Button>
          </Space>
          {query.isError && (
            <Alert
              showIcon
              type="error"
              title={errorMessage(query.error)}
              description="上次快照可能已过期，请刷新重试。"
            />
          )}
          {query.isPending && <Spin />}
          <Tabs
            items={[
              {
                key: 'detail',
                label: '存储详情',
                children: query.data && <StorageDetails document={query.data} target={target} />,
              },
              {
                key: 'relations',
                label: '关联资源与事件',
                children: (
                  <RelationsPanel
                    key={JSON.stringify(target)}
                    target={target}
                    contextName={profile.context}
                  />
                ),
              },
            ]}
          />
        </>
      )}
    </Drawer>
  );
}
