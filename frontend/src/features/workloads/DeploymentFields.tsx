import { Alert, Input, InputNumber } from 'antd';
import { Fields } from '../visual-forms/Fields';
import { podOptions } from '../visual-forms/podSchema';
import { valueOf } from '../custom-resources/values';
import type { Deployment } from '../../api/types';
import { NamespaceSelect } from '../resources/NamespaceSelect';
import { ContainerList } from './ContainerList';
import { FieldGroups } from '../visual-forms/FieldGroups';
import { podGroups } from '../visual-forms/groupDefinitions';
import type { ReactNode } from 'react';

import type { WorkloadAction } from './actions';

export function DeploymentFields({
  clusterID,
  action,
  value,
  onChange,
  kind = 'Deployment',
  showReplicas = true,
  nodeSelector,
}: {
  kind?: string;
  nodeSelector?: ReactNode;
  showReplicas?: boolean;
  clusterID: string;
  action: WorkloadAction;
  value: Deployment;
  onChange: (value: Deployment) => void;
}) {
  const set = (patch: Partial<Deployment>) => onChange({ ...value, ...patch });
  const containers = value.containers ?? [];
  if (action === 'pause' || action === 'resume')
    return (
      <Alert
        type="warning"
        showIcon
        title={
          action === 'pause'
            ? '暂停后控制器暂不发布新的 Pod 模板变更。'
            : '恢复后控制器将按当前 Pod 模板继续发布。'
        }
        description="只修改 spec.paused。不会停止已有 Pod；请在提交后检查发布状态。"
      />
    );
  if (action === 'restart')
    return (
      <Alert
        type="warning"
        showIcon
        title="重启将更新 Pod 模板时间戳，并按现有发布策略替换 Pod。"
        description="RollingUpdate 按滚动策略执行，Recreate 会先终止旧 Pod；提交后请在详情中查看发布进度。"
      />
    );
  return (
    <>
      <div className="workload-fields">
        {action === 'create' && (
          <>
            <label>
              {kind} 名称
              <Input
                aria-label={`${kind} 名称`}
                value={value.name}
                onChange={(event) => set({ name: event.target.value })}
                placeholder="例如 api-gateway"
              />
            </label>
            <label>
              命名空间
              <NamespaceSelect
                clusterID={clusterID}
                allowAll={false}
                value={value.namespace}
                onChange={(namespace) => set({ namespace })}
              />
            </label>
          </>
        )}
        {showReplicas && (
          <label>
            期望副本数
            <InputNumber
              aria-label="期望副本数"
              min={0}
              max={2147483647}
              precision={0}
              value={value.replicas}
              onChange={(replicas) => {
                if (replicas !== null) set({ replicas });
              }}
            />
          </label>
        )}
      </div>
      {showReplicas && (
        <p className="muted workload-note">
          副本数为 0 会停止全部实例；如果配置了 HPA，后续可能由 HPA 调整副本数。
        </p>
      )}
      <Alert
        type="info"
        showIcon
        title={
          action === 'create'
            ? '自动生成 app.kubernetes.io/name 标签和选择器，发布策略由集群默认。'
            : '支持容器、初始化容器、卷、调度和安全配置；表单未展示的字段保留。'
        }
      />
      <ContainerList value={containers} onChange={(next) => set({ containers: next })} />
      <section className="pod-configuration" aria-label="Pod 配置">
        <div className="configuration-section-heading">
          <div className="container-section-heading">
            <h3>Pod 配置</h3>
            <p className="muted">调度、存储和安全设置对该工作负载的全部容器生效。</p>
          </div>
        </div>
        <FieldGroups
          groups={podGroups}
          leading={{ scheduling: nodeSelector }}
          fields={
            kind === 'Deployment'
              ? podOptions
              : podOptions.filter((field) => field.key !== 'nodeSelector')
          }
          value={value.podOptions ?? valueOf('object')}
          onChange={(podOptions) => set({ podOptions })}
          renderFields={(props) => <Fields {...props} />}
        />
      </section>
    </>
  );
}
