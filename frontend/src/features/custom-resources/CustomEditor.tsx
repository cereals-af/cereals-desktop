import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Drawer, Input, Segmented, Spin } from 'antd';
import { useAPI } from '../../api/context';
import type { CustomSnapshot, CustomValue, Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { FormSession } from '../resources/FormSession';
import { NamespaceSelect } from '../resources/NamespaceSelect';
import { DynamicFields } from './DynamicFields';
import { identity, setIdentity } from './values';
const YAMLEditor = lazy(() => import('../resources/SchemaEditor'));
type EditorDraft = { mode: 'form' | 'yaml'; value: CustomValue; yaml: string };
interface Props {
  target: Ref;
  profile: Profile;
  namespaced: boolean;
  onClose: () => void;
  onSuccess: () => void;
}
export function CustomEditor(props: Props) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', props.target.clusterID, 'custom-form', props.target],
    queryFn: ({ signal }) => api.customSnapshot(props.target, signal),
    gcTime: 0,
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  });
  return (
    <Drawer
      open
      size={1080}
      destroyOnHidden
      onClose={props.onClose}
      className="manifest-drawer"
      title={
        <div className="drawer-title">
          <span>{props.target.name ? `编辑实例 · ${props.target.name}` : '创建自定义资源'}</span>
          <small>
            {props.profile.context} /{' '}
            {props.target.namespace || (props.namespaced ? '待选择命名空间' : '集群作用域')} ·{' '}
            {props.target.group}/{props.target.version}
          </small>
        </div>
      }
    >
      {query.isPending && <Spin />}
      {query.isError && (
        <Alert
          type="error"
          showIcon
          title={errorMessage(query.error)}
          action={<Button onClick={() => void query.refetch()}>重试读取表单</Button>}
        />
      )}
      {query.data && <EditorSession {...props} snapshot={query.data} />}
    </Drawer>
  );
}
function EditorSession(props: Props & { snapshot: CustomSnapshot }) {
  const api = useAPI();
  const initial: EditorDraft = { mode: 'form', ...props.snapshot.draft };
  return (
    <FormSession
      initial={initial}
      baseline={props.snapshot.baseline}
      creating={!props.target.name}
      prepare={async (draft, baseline, signal) => {
        const value =
          !props.target.name && props.namespaced && draft.mode === 'yaml'
            ? (await api.convertCustom(null, draft.yaml, signal)).value
            : draft.value;
        return api.previewCustom(
          {
            target: {
              ...props.target,
              namespace:
                !props.target.name && props.namespaced
                  ? identity(value, 'namespace')
                  : props.target.namespace,
            },
            baseline,
            definition: props.snapshot.definition,
            mode: props.target.name ? 'update' : 'create',
            value: draft.mode === 'form' ? draft.value : null,
            yaml: draft.mode === 'yaml' ? draft.yaml : '',
          },
          signal,
        );
      }}
      readBaseline={async (signal) => {
        const next = await api.customSnapshot(props.target, signal);
        if (
          next.definition.uid !== props.snapshot.definition.uid ||
          next.definition.resourceVersion !== props.snapshot.definition.resourceVersion
        )
          throw new Error('CRD 定义已变化，请保留 YAML 草稿，关闭并重新打开表单');
        return next.baseline;
      }}
      onSuccess={props.onSuccess}
      onDone={props.onClose}
      onBack={props.onClose}
      doneLabel="返回实例列表"
    >
      {(draft, setDraft) => <DraftFields {...props} draft={draft} onChange={setDraft} />}
    </FormSession>
  );
}
function DraftFields(
  props: Props & {
    snapshot: CustomSnapshot;
    draft: EditorDraft;
    onChange: (draft: EditorDraft) => void;
  },
) {
  const api = useAPI();
  const [converting, setConverting] = useState(false);
  const [error, setError] = useState('');
  const lifetime = useRef(new AbortController());
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    return () => controller.abort();
  }, []);
  const switchMode = async (mode: 'form' | 'yaml') => {
    if (mode === props.draft.mode || converting) return;
    const signal = lifetime.current.signal;
    setConverting(true);
    setError('');
    try {
      const result = await api.convertCustom(
        props.draft.mode === 'form' ? props.draft.value : null,
        props.draft.mode === 'yaml' ? props.draft.yaml : '',
        signal,
      );
      if (!signal.aborted) props.onChange({ ...result, mode });
    } catch (failure) {
      if (!signal.aborted) setError(errorMessage(failure));
    } finally {
      if (!signal.aborted) setConverting(false);
    }
  };
  const value = props.draft.value;
  const changeValue = (value: CustomValue) => props.onChange({ ...props.draft, value });
  return (
    <>
      <Alert
        type="info"
        showIcon
        title="按当前 CRD Schema 编辑；未展示字段完整保留"
        description="表单与 YAML 转换在 Go 中完成，整数以文本保存。服务端执行必填、范围、CEL、默认值及权限检查；定义变化后需重新加载表单。"
      />
      {error && <Alert type="error" showIcon title={error} />}
      <div className="mutation-toolbar">
        <Segmented
          aria-label="自定义资源编辑模式"
          disabled={converting}
          value={props.draft.mode}
          options={[
            { value: 'form', label: '可视化表单' },
            { value: 'yaml', label: 'YAML 草稿' },
          ]}
          onChange={(mode) => {
            if (mode === 'form' || mode === 'yaml') void switchMode(mode);
          }}
        />
        {converting && <Spin size="small" />}
      </div>
      {converting ? (
        <Spin />
      ) : props.draft.mode === 'yaml' ? (
        <Suspense fallback={<Spin />}>
          <YAMLEditor
            target={props.target}
            value={props.draft.yaml}
            readOnly={converting}
            onChange={(yaml) => props.onChange({ ...props.draft, yaml })}
          />
        </Suspense>
      ) : (
        <>
          <div className="workload-fields">
            <label>
              实例名称
              <Input
                aria-label="自定义资源名称"
                disabled={Boolean(props.target.name) || converting}
                value={identity(value, 'name')}
                onChange={(event) => changeValue(setIdentity(value, 'name', event.target.value))}
              />
            </label>
            {props.namespaced && (
              <label>
                命名空间
                {props.target.name ? (
                  <Input aria-label="实例命名空间" readOnly value={identity(value, 'namespace')} />
                ) : (
                  <NamespaceSelect
                    clusterID={props.target.clusterID}
                    value={identity(value, 'namespace')}
                    allowAll={false}
                    onChange={(namespace) =>
                      changeValue(setIdentity(value, 'namespace', namespace))
                    }
                  />
                )}
              </label>
            )}
          </div>
          {props.snapshot.fields?.length ? (
            <DynamicFields fields={props.snapshot.fields} value={value} onChange={changeValue} />
          ) : (
            <Alert type="warning" title="此 Schema 没有可视化字段，请使用 YAML 草稿" />
          )}
        </>
      )}
    </>
  );
}
