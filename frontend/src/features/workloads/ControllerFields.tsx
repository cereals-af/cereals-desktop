import { Alert, Input, InputNumber, Select } from 'antd';
import type { ControllerForm } from '../../api/types';
import { DeploymentFields } from './DeploymentFields';
import { ClaimTemplates } from './ClaimTemplates';
import { strategyNote } from './controllers';
import { NodeSelectorFields } from './NodeSelectorFields';

export function ControllerFields({
  clusterID,
  resource,
  kind,
  action,
  value,
  onChange,
}: {
  clusterID: string;
  resource: string;
  kind: string;
  action: 'create' | 'update' | 'restart';
  value: ControllerForm;
  onChange: (form: ControllerForm) => void;
}) {
  const set = (patch: Partial<ControllerForm>) => onChange({ ...value, ...patch });
  const creating = action === 'create';
  const stateful = resource === 'statefulsets';
  if (action === 'restart')
    return (
      <Alert
        showIcon
        type="warning"
        title="更新 Pod 模板的重启时间戳"
        description={strategyNote(resource, value)}
      />
    );
  return (
    <>
      <Alert showIcon type="info" title={strategyNote(resource, value)} />
      <DeploymentFields
        kind={kind}
        clusterID={clusterID}
        action={action}
        showReplicas={creating && resource !== 'daemonsets'}
        nodeSelector={
          <NodeSelectorFields
            value={value.nodeSelector ?? []}
            onChange={(nodeSelector) => set({ nodeSelector })}
          />
        }
        value={value.workload}
        onChange={(workload) => set({ workload })}
      />
      {resource !== 'replicasets' && (
        <section className="configuration-block">
          <h3>更新策略</h3>
          <div className="workload-fields">
            <label>
              策略
              <Select
                aria-label="更新策略"
                value={value.strategy}
                onChange={(strategy) => set({ strategy })}
                options={[
                  'RollingUpdate',
                  'OnDelete',
                  ...(!['RollingUpdate', 'OnDelete'].includes(value.strategy)
                    ? [value.strategy]
                    : []),
                ].map((v) => ({ value: v, label: v }))}
              />
            </label>
            {value.strategy === 'RollingUpdate' &&
              (stateful ? (
                <label>
                  分区 partition
                  <InputNumber
                    aria-label="更新分区"
                    min={0}
                    max={2147483647}
                    precision={0}
                    value={value.partition}
                    onChange={(partition) => set({ partition: partition ?? 0 })}
                  />
                </label>
              ) : (
                <>
                  <label>
                    最大不可用
                    <Input
                      aria-label="最大不可用"
                      value={value.maxUnavailable}
                      onChange={(e) => set({ maxUnavailable: e.target.value })}
                    />
                  </label>
                  <label>
                    最大额外 Pod
                    <Input
                      aria-label="最大额外 Pod"
                      value={value.maxSurge}
                      onChange={(e) => set({ maxSurge: e.target.value })}
                    />
                  </label>
                </>
              ))}
          </div>
          {!stateful && (
            <p className="muted">
              整数或百分比；maxUnavailable 与 maxSurge 不能同时为零，也不能同时非零。API Server
              预检确认策略有效性。
            </p>
          )}
        </section>
      )}
      {stateful && (
        <section className="configuration-block">
          <h3>稳定身份与存储</h3>
          <Alert
            showIcon
            type="info"
            title="治理 Service、Pod 管理顺序和 PVC 模板创建后不可修改"
            description="治理 Service 通常是预先创建的 Headless Service。已有 PVC 的扩容请进入关联 PVC 的存储详情；创建工作负载时，模板挂载应用于全部常规容器，后续新增容器的挂载可通过 YAML 配置。"
          />
          <div className="workload-fields">
            <label>
              治理 Service
              <Input
                aria-label="治理 Service"
                disabled={!creating}
                value={value.serviceName}
                onChange={(e) => set({ serviceName: e.target.value })}
              />
            </label>
            <label>
              Pod 管理顺序
              <Select
                aria-label="Pod 管理顺序"
                disabled={!creating}
                value={value.podManagementPolicy}
                onChange={(podManagementPolicy) => set({ podManagementPolicy })}
                options={['OrderedReady', 'Parallel'].map((v) => ({ value: v, label: v }))}
              />
            </label>
            {(['retainDeleted', 'retainScaled'] as const).map((field) => (
              <label key={field}>
                {field === 'retainDeleted' ? '删除 STS 时的 PVC' : '缩容时的 PVC'}
                <Select
                  aria-label={field === 'retainDeleted' ? '删除时 PVC 策略' : '缩容时 PVC 策略'}
                  value={value[field]}
                  onChange={(v) => set({ [field]: v })}
                  options={[
                    { value: 'Retain', label: 'Retain · 保留' },
                    { value: 'Delete', label: 'Delete · 删除' },
                  ]}
                />
              </label>
            ))}
          </div>
          {(value.retainScaled === 'Delete' || value.retainDeleted === 'Delete') && (
            <Alert
              type="warning"
              showIcon
              title="Delete 策略会让控制器删除符合条件的 PVC，底层数据是否删除取决于 PV 回收策略。请检查差异后提交。"
            />
          )}
          <ClaimTemplates
            clusterID={clusterID}
            value={value.claims ?? []}
            creating={creating}
            onChange={(claims) => set({ claims })}
          />
        </section>
      )}
    </>
  );
}
