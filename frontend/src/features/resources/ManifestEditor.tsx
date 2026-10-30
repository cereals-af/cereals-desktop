import { lazy, Suspense, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Checkbox, Drawer, Select, Space, Spin } from 'antd';
import { useAPI } from '../../api/context';
import type { Profile, Ref, ResourceType, Snapshot } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { MutationReview } from './MutationReview';
import { useManifestDraft } from './useManifestDraft';

const Editor = lazy(() => import('./SchemaEditor'));
interface Props {
  target: Ref;
  type: ResourceType;
  profile: Profile;
  onClose: () => void;
  onSuccess: () => void;
}

export function ManifestEditor(props: Props) {
  const api = useAPI();
  const secret = props.target.group === '' && props.target.resource === 'secrets';
  const [reveal, setReveal] = useState(false);
  const allowed = !secret || reveal;
  const query = useQuery({
    queryKey: ['cluster', props.target.clusterID, 'edit', props.target, reveal],
    queryFn: ({ signal }) => api.snapshot(props.target, reveal, signal),
    enabled: Boolean(props.target.name) && allowed,
    gcTime: 0,
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const snapshot = props.target.name ? query.data : newSnapshot(props.target, props.type);
  return (
    <Drawer
      open
      size={1080}
      className="manifest-drawer"
      destroyOnHidden
      onClose={props.onClose}
      title={`${props.target.name ? '编辑' : '创建'} ${props.type.kind} · ${props.profile.context}`}
    >
      <p className="muted">
        {props.profile.server} · {props.target.namespace || '由 YAML 指定命名空间 / 集群作用域'}
      </p>
      {secret && (
        <Alert
          type="warning"
          showIcon
          title="Secret 内容仅在当前编辑会话中显示，请勿粘贴到日志或分享截图。"
          action={
            <Checkbox checked={reveal} onChange={(event) => setReveal(event.target.checked)}>
              显示并编辑敏感内容
            </Checkbox>
          }
        />
      )}
      {query.isError && <Alert type="error" title={errorMessage(query.error)} showIcon />}
      {allowed &&
        (snapshot ? (
          <EditorSession key={String(reveal)} {...props} snapshot={snapshot} reveal={reveal} />
        ) : (
          !query.isError && <Spin />
        ))}
    </Drawer>
  );
}

function newSnapshot(target: Ref, type: ResourceType): Snapshot {
  const version = type.group ? `${type.group}/${type.version}` : type.version;
  return {
    editable: true,
    baseline: { uid: '', resourceVersion: '' },
    yaml: `apiVersion: ${version}\nkind: ${type.kind}\nmetadata:\n  name: new-resource\n${type.namespaced ? `  namespace: ${target.namespace || 'default'}\n` : ''}`,
  };
}

function EditorSession({
  target,
  snapshot,
  reveal,
  onSuccess,
}: Props & { snapshot: Snapshot; reveal: boolean }) {
  const edit = useManifestDraft(target, snapshot, reveal, onSuccess);
  const busy = edit.phase.kind === 'previewing' || edit.phase.kind === 'committing';
  if (edit.phase.kind === 'done')
    return <Alert type="success" showIcon title="请求已提交" description={edit.phase.message} />;
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
            <Button type="primary" loading={busy} onClick={() => void edit.commit()}>
              确认提交到集群
            </Button>
          </div>
        </>
      ) : (
        <>
          <div className="mutation-toolbar">
            <Space>
              <Select
                aria-label="写入方式"
                value={edit.mode}
                disabled={busy || !target.name}
                onChange={edit.setMode}
                options={
                  target.name
                    ? [
                        { value: 'patch', label: '差量更新（Merge Patch）' },
                        { value: 'apply', label: '声明式应用（SSA）' },
                      ]
                    : [{ value: 'create', label: '创建资源' }]
                }
              />
              {target.name && (
                <Button disabled={busy} onClick={() => void edit.reload()}>
                  重新读取基线（保留草稿）
                </Button>
              )}
            </Space>
          </div>
          <Alert
            showIcon
            type="info"
            title={
              edit.mode === 'apply'
                ? 'SSA：仅保留希望由 cereals 管理的字段；省略此前由 cereals 管理的字段可能删除它们。字段冲突不会强制覆盖。兼容已有资源，字段管理器仍为 creals。'
                : '编辑完整 YAML，未修改的未知字段会保留；删除字段代表明确移除。status 和服务器管理的元数据不参与写入。'
            }
          />
          <Suspense fallback={<Spin />}>
            <Editor target={target} value={edit.draft} readOnly={busy} onChange={edit.setDraft} />
          </Suspense>
          <div className="mutation-actions">
            <span className="muted">单个 YAML 文档 · 最大 2 MiB · 草稿仅保存在当前窗口内存中</span>
            <Button type="primary" loading={busy} onClick={() => void edit.preview()}>
              服务端预检并查看差异
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
