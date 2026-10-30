import { Button, Collapse, Input, Tabs } from 'antd';
import { Fields } from '../visual-forms/Fields';
import { containerAdvanced } from '../visual-forms/podSchema';
import { valueOf } from '../custom-resources/values';
import type { WorkloadContainer } from '../../api/types';
import { ProbeFields } from './ProbeFields';
import { ContainerPorts } from './ContainerPorts';
import { ContainerProcess } from './ContainerProcess';

export function ContainerFields({
  value,
  canRemove,
  onRemove,
  onChange,
}: {
  value: WorkloadContainer;
  canRemove: boolean;
  onRemove: () => void;
  onChange: (value: WorkloadContainer) => void;
}) {
  const set = (patch: Partial<WorkloadContainer>) => onChange({ ...value, ...patch });
  const advancedFields = (keys: string[]) => (
    <Fields
      fields={containerAdvanced.filter((field) => keys.includes(field.key))}
      value={value.advanced ?? valueOf('object')}
      onChange={(advanced) => set({ advanced })}
    />
  );
  return (
    <section className="workload-container" aria-label={`容器配置 ${value.name || '未命名'}`}>
      <div className="workload-container-header">
        <h3>容器 · {value.name || '未命名'}</h3>
        <Button className="form-item-action" disabled={!canRemove} onClick={onRemove}>
          移除此容器
        </Button>
      </div>
      <div className="workload-fields">
        <label>
          容器名称
          <Input
            aria-label="容器名称"
            disabled={Boolean(value.originalName)}
            value={value.name}
            onChange={(event) => set({ name: event.target.value })}
          />
        </label>
        <label>
          容器镜像
          <Input
            aria-label="容器镜像"
            value={value.image}
            placeholder="nginx:1.29"
            onChange={(event) => set({ image: event.target.value })}
          />
        </label>
      </div>
      <Tabs
        className="configuration-tabs"
        items={[
          {
            key: 'basic',
            label: '基础配置',
            children: (
              <>
                <section className="container-section">
                  <h4>资源配额</h4>
                  <div className="workload-fields">
                    {(
                      [
                        ['cpuRequest', 'CPU 请求', '100m'],
                        ['cpuLimit', 'CPU 上限', '1'],
                        ['memoryRequest', '内存请求', '128Mi'],
                        ['memoryLimit', '内存上限', '512Mi'],
                      ] as const
                    ).map(([key, label, placeholder]) => (
                      <label key={key}>
                        {label}
                        <Input
                          aria-label={label}
                          value={value[key]}
                          placeholder={placeholder}
                          onChange={(event) => set({ [key]: event.target.value })}
                        />
                      </label>
                    ))}
                  </div>
                </section>
                <ContainerProcess
                  command={value.command ?? []}
                  args={value.args ?? []}
                  onChange={set}
                />
                {advancedFields(['workingDir', 'imagePullPolicy'])}
              </>
            ),
          },
          {
            key: 'ports',
            label: '端口',
            children: (
              <ContainerPorts ports={value.ports ?? []} onChange={(ports) => set({ ports })} />
            ),
          },
          { key: 'environment', label: '环境变量', children: advancedFields(['env', 'envFrom']) },
          {
            key: 'storage',
            label: '存储挂载',
            children: advancedFields(['volumeMounts', 'volumeDevices']),
          },
          {
            key: 'health',
            label: '健康检查',
            children: (
              <Collapse
                className="probe-collapse"
                items={(
                  [
                    ['readiness', '就绪探针'],
                    ['liveness', '存活探针'],
                    ['startup', '启动探针'],
                  ] as const
                ).map(([key, label]) => ({
                  key,
                  label: `${label} · ${value[key].kind}`,
                  children: (
                    <ProbeFields
                      value={value[key]}
                      readiness={key === 'readiness'}
                      onChange={(probe) => set({ [key]: probe })}
                    />
                  ),
                }))}
              />
            ),
          },
          { key: 'security', label: '安全设置', children: advancedFields(['securityContext']) },
        ]}
      />
    </section>
  );
}
