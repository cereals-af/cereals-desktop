import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Drawer, Space, Spin, Tabs } from 'antd';
import { RelationsPanel } from '../relations/RelationsPanel';
import { useAPI } from '../../api/context';
import type { Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { CoreEditor } from './CoreEditor';
import { CoreDetails } from './CoreDetails';
import { loadDocument, newDocument } from './models';
import type { FormDocument, FormKind } from './models';

type View = { kind: 'detail' } | { kind: 'edit'; document: FormDocument };

export function CoreWorkspace({
  target: initialTarget,
  kind,
  profile,
  canPatch,
  onClose,
  onYAML,
  onSuccess,
}: {
  target: Ref;
  kind: FormKind;
  profile: Profile;
  canPatch: boolean;
  onClose: () => void;
  onYAML: (target: Ref) => void;
  onSuccess: () => void;
}) {
  const api = useAPI();
  const [target, setTarget] = useState(initialTarget);
  const [revision, setRevision] = useState(0);
  const [view, setView] = useState<View>(() =>
    target.name
      ? { kind: 'detail' }
      : { kind: 'edit', document: newDocument(kind, target.namespace || profile.namespace) },
  );
  const label = kind === 'service' ? 'Service' : 'ConfigMap';
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'core-form', target, revision],
    queryFn: ({ signal }) => loadDocument(api, kind, target, signal),
    enabled: view.kind === 'detail' && Boolean(target.name),
    gcTime: 0,
    retry: false,
  });
  const done = (next: Ref) => {
    setTarget(next);
    setRevision((current) => current + 1);
    setView({ kind: 'detail' });
  };
  return (
    <Drawer
      open
      destroyOnHidden
      size={1080}
      className="manifest-drawer core-workspace"
      onClose={onClose}
      title={
        <div className="drawer-title">
          <span>
            {!target.name ? `创建 ${label}` : `${label} · ${target.name}`}
            {view.kind === 'edit' && target.name ? ' · 可视化编辑' : ''}
          </span>
          <small>
            {profile.context} / {target.name ? target.namespace : '新建资源 · 选择命名空间'} ·{' '}
            {profile.server}
          </small>
        </div>
      }
    >
      {view.kind === 'edit' ? (
        <CoreEditor
          document={view.document}
          target={target}
          onSuccess={onSuccess}
          onDone={done}
          onBack={() => setView({ kind: 'detail' })}
        />
      ) : (
        <>
          <Space wrap className="mutation-toolbar">
            <Button onClick={() => onYAML(target)}>查看 / 编辑 YAML</Button>
            {canPatch && (
              <Button
                type="primary"
                disabled={
                  !query.data ||
                  query.isError ||
                  query.isFetching ||
                  (query.data.kind === 'configmap' && query.data.snapshot.form.immutable)
                }
                onClick={() => {
                  if (query.data) setView({ kind: 'edit', document: query.data });
                }}
              >
                可视化编辑
              </Button>
            )}
            <Button loading={query.isFetching} onClick={() => void query.refetch()}>
              刷新详情
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
                label: '资源详情',
                children: query.data && <CoreDetails document={query.data} />,
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
