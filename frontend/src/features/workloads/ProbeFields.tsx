import { Input, InputNumber, Select } from 'antd';
import type { Probe } from '../../api/types';

interface Props {
  value: Probe;
  onChange: (probe: Probe) => void;
  readiness: boolean;
}

export function ProbeFields({ value, onChange, readiness }: Props) {
  const set = (patch: Partial<Probe>) => onChange({ ...value, ...patch });
  return (
    <div className="workload-fields">
      <label>
        探针类型
        <Select
          aria-label="探针类型"
          value={value.kind}
          onChange={(kind) => set({ kind })}
          options={[
            { value: 'none', label: '不设置' },
            { value: 'http', label: 'HTTP / HTTPS' },
            { value: 'tcp', label: 'TCP' },
            { value: 'exec', label: 'Exec' },
            ...(value.kind === 'keep'
              ? [{ value: 'keep', label: '保留现有探针（通过 YAML 编辑）' }]
              : []),
          ]}
        />
      </label>
      {value.kind !== 'none' && value.kind !== 'keep' && (
        <>
          {value.kind === 'http' && (
            <>
              <label>
                路径
                <Input
                  aria-label="探针路径"
                  value={value.path}
                  onChange={(event) => set({ path: event.target.value })}
                />
              </label>
              <label>
                协议
                <Select
                  value={value.scheme}
                  onChange={(scheme) => set({ scheme })}
                  options={['HTTP', 'HTTPS'].map((label) => ({ value: label, label }))}
                />
              </label>
            </>
          )}
          {(value.kind === 'http' || value.kind === 'tcp') && (
            <label>
              端口 / 端口名称
              <Input
                aria-label="探针端口"
                value={value.port}
                onChange={(event) => set({ port: event.target.value })}
                placeholder="8080 或 http"
              />
            </label>
          )}
          {value.kind === 'exec' && (
            <label className="wide-field">
              命令（每行一个参数，不经过 shell）
              <Input.TextArea
                aria-label="探针命令"
                value={(value.command ?? []).join('\n')}
                onChange={(event) => set({ command: event.target.value.split('\n') })}
                rows={3}
              />
            </label>
          )}
          {(
            [
              ['initialDelay', '初始延迟（秒）', 0],
              ['timeout', '超时（秒）', 1],
              ['period', '周期（秒）', 1],
              ['failureThreshold', '失败阈值', 1],
              ...(readiness ? [['successThreshold', '成功阈值', 1] as const] : []),
            ] as const
          ).map(([key, label, min]) => (
            <label key={key}>
              {label}
              <InputNumber
                min={min}
                max={2147483647}
                precision={0}
                value={value[key]}
                onChange={(number) => {
                  if (number !== null) set({ [key]: number });
                }}
              />
            </label>
          ))}
        </>
      )}
    </div>
  );
}
