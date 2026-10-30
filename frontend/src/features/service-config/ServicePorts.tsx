import { Button, Input, InputNumber, Select } from 'antd';
import type { ServicePort } from '../../api/types';
import { newPort } from './models';

export function ServicePorts({
  ports,
  type,
  onChange,
}: {
  ports: ServicePort[];
  type: string;
  onChange: (ports: ServicePort[]) => void;
}) {
  const nodePort = type === 'NodePort' || type === 'LoadBalancer';
  const update = (index: number, patch: Partial<ServicePort>) =>
    onChange(ports.map((port, i) => (i === index ? { ...port, ...patch } : port)));
  return (
    <section className="service-ports">
      <h3>端口映射</h3>
      {ports.map((port, index) => (
        <div className="service-port" key={index}>
          <div className="workload-fields">
            <label>
              端口名称
              <Input
                aria-label={`端口名称 ${index + 1}`}
                value={port.name}
                onChange={(event) => update(index, { name: event.target.value })}
              />
            </label>
            <label>
              传输协议
              <Select
                aria-label={`端口协议 ${index + 1}`}
                value={port.protocol}
                onChange={(protocol) => update(index, { protocol })}
                options={['TCP', 'UDP', 'SCTP'].map((value) => ({ value, label: value }))}
              />
            </label>
            <label>
              Service 端口
              <InputNumber
                aria-label={`Service 端口 ${index + 1}`}
                min={1}
                max={65535}
                precision={0}
                value={port.port}
                onChange={(value) => {
                  if (value !== null) update(index, { port: value });
                }}
              />
            </label>
            <label>
              目标端口 / 端口名称
              <Input
                aria-label={`目标端口 ${index + 1}`}
                value={port.targetPort}
                placeholder="例如 8080 或 http；留空跟随 Service 端口"
                onChange={(event) => update(index, { targetPort: event.target.value })}
              />
            </label>
            {nodePort && (
              <label>
                节点端口（0 自动分配）
                <InputNumber
                  aria-label={`节点端口 ${index + 1}`}
                  min={0}
                  max={65535}
                  precision={0}
                  value={port.nodePort}
                  onChange={(value) => {
                    if (value !== null) update(index, { nodePort: value });
                  }}
                />
              </label>
            )}
            <label>
              应用协议（可选）
              <Input
                aria-label={`应用协议 ${index + 1}`}
                value={port.appProtocol}
                placeholder="例如 http 或 kubernetes.io/h2c"
                onChange={(event) => update(index, { appProtocol: event.target.value })}
              />
            </label>
          </div>
          <Button onClick={() => onChange(ports.filter((_, i) => i !== index))}>移除此端口</Button>
        </div>
      ))}
      <Button
        onClick={() => onChange([...ports, { ...newPort(), name: `port-${ports.length + 1}` }])}
      >
        添加端口
      </Button>
    </section>
  );
}
