import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Descriptions, Drawer, Input, InputNumber, Select, Spin } from 'antd';
import { useAPI } from '../../api/context';
import type { Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { FormSession } from '../resources/FormSession';
import { NamespaceSelect } from '../resources/NamespaceSelect';
import { ScaleStorageNotice } from './ScaleStorageNotice';
import { WorkloadAccess } from './WorkloadAccess';

interface Props {
  target: Ref;
  profile: Profile;
  onClose: () => void;
  onSuccess: () => void;
  onStatus?: (target: Ref) => void;
}
export function ScaleWorkspace(props: Props) {
  const [target, setTarget] = useState<Ref | null>(props.target.name ? props.target : null);
  return (
    <Drawer
      open
      size={980}
      destroyOnHidden
      onClose={props.onClose}
      title={
        <div className="drawer-title">
          <span>扩缩容 · {target?.name || '按名称访问'}</span>
          <small>
            {props.profile.context} /{' '}
            {target?.namespace || props.target.namespace || '选择命名空间'} ·{' '}
            {props.target.resource}/scale
          </small>
        </div>
      }
    >
      {target ? (
        <ScaleDetails key={JSON.stringify(target)} {...props} target={target} />
      ) : (
        <ScaleTarget target={props.target} onSelect={setTarget} />
      )}
    </Drawer>
  );
}
function ScaleTarget({ target, onSelect }: { target: Ref; onSelect: (value: Ref) => void }) {
  const [namespace, setNamespace] = useState(target.namespace);
  const [name, setName] = useState('');
  return (
    <>
      <Alert
        type="info"
        showIcon
        title="可直接访问 scale，无需列出或读取父资源"
        description="请填写已有工作负载的名称和命名空间；读取需要 get，提交需要 update 或 patch 子资源权限。"
      />
      <div className="workload-fields">
        <label>
          命名空间
          <NamespaceSelect
            clusterID={target.clusterID}
            allowAll={false}
            value={namespace}
            onChange={setNamespace}
          />
        </label>
        <label>
          工作负载名称
          <Input
            aria-label="扩缩容工作负载名称"
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
      </div>
      <Button
        type="primary"
        disabled={!name.trim() || !namespace.trim()}
        onClick={() => onSelect({ ...target, name: name.trim(), namespace: namespace.trim() })}
      >
        读取 Scale
      </Button>
    </>
  );
}
function ScaleDetails(props: Props) {
  const api = useAPI();
  const [revision, setRevision] = useState(0);
  const query = useQuery({
    queryKey: ['cluster', props.target.clusterID, 'scale', props.target, revision],
    queryFn: ({ signal }) => api.scale(props.target, signal),
    retry: false,
    gcTime: 0,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
  const done = (target: Ref) => {
    if (props.onStatus) props.onStatus(target);
    else setRevision((value) => value + 1);
  };
  return (
    <>
      <WorkloadAccess target={props.target} scale />
      {query.isPending && <Spin />}
      {query.isError && (
        <Alert
          type="error"
          showIcon
          title={errorMessage(query.error)}
          action={<Button onClick={() => void query.refetch()}>重试读取 Scale</Button>}
        />
      )}
      {query.data && (
        <FormSession
          key={revision}
          initial={{ replicas: query.data.replicas, method: 'update' }}
          baseline={query.data.baseline}
          creating={false}
          prepare={(form, baseline, signal) =>
            api.previewScale({ target: props.target, baseline, ...form }, signal)
          }
          readBaseline={async (signal) => (await api.scale(props.target, signal)).baseline}
          onSuccess={props.onSuccess}
          onDone={done}
          onBack={props.onClose}
          doneLabel={props.onStatus ? '查看发布状态' : '重新读取 Scale'}
        >
          {(form, setForm) => (
            <>
              <Descriptions
                size="small"
                items={[
                  { key: 'actual', label: '已报告副本', children: query.data?.actual },
                  { key: 'uid', label: '资源 UID', children: query.data?.baseline.uid },
                  {
                    key: 'selector',
                    label: 'Pod 选择器',
                    children: query.data?.selector || '未提供',
                  },
                ]}
              />
              <Alert
                type="warning"
                showIcon
                title="副本数为 0 会停止全部实例；HPA 可能再次调整副本数。"
                description="仅修改 scale.spec.replicas，使用 Scale 的 UID/resourceVersion 进行并发保护。"
              />
              <ScaleStorageNotice target={props.target} replicas={form.replicas} />
              <div className="workload-fields">
                <label>
                  期望副本数
                  <InputNumber
                    aria-label="期望副本数"
                    min={0}
                    max={2147483647}
                    precision={0}
                    value={form.replicas}
                    onChange={(value) => {
                      if (value !== null) setForm({ ...form, replicas: value });
                    }}
                  />
                </label>
                <label>
                  提交方式
                  <Select
                    aria-label="Scale 提交方式"
                    value={form.method}
                    onChange={(method) => setForm({ ...form, method })}
                    options={[
                      { value: 'update', label: '更新 · 需要 update scale' },
                      { value: 'patch', label: '补丁 · 需要 patch scale' },
                    ]}
                  />
                </label>
              </div>
            </>
          )}
        </FormSession>
      )}
    </>
  );
}
