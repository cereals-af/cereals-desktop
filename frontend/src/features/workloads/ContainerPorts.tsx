import { useState } from 'react';
import { Button, Input, InputNumber, Select } from 'antd';
import type { ContainerPort } from '../../api/types';

export function ContainerPorts({
  ports,
  onChange,
}: {
  ports: ContainerPort[];
  onChange: (ports: ContainerPort[]) => void;
}) {
  const [keys, setKeys] = useState(() => ports.map(() => crypto.randomUUID()));
  const update = (index: number, patch: Partial<ContainerPort>) =>
    onChange(ports.map((port, i) => (i === index ? { ...port, ...patch } : port)));
  return (
    <section className="container-section container-ports">
      <div className="container-section-heading">
        <h4>端口映射</h4>
        <p className="muted">声明容器端口；主机端口与 IP 按需填写，留空时不映射主机端口。</p>
      </div>
      {ports.map((port, index) => (
        <div className="configuration-row container-port-row" key={keys[index]}>
          <label>
            名称（可选）
            <Input
              aria-label={`容器端口名称 ${index + 1}`}
              value={port.name}
              placeholder="例如 http"
              onChange={(event) => update(index, { name: event.target.value })}
            />
          </label>
          <label>
            协议
            <Select
              aria-label={`容器端口协议 ${index + 1}`}
              value={port.protocol}
              options={['TCP', 'UDP', 'SCTP'].map((value) => ({ value, label: value }))}
              onChange={(protocol) => update(index, { protocol })}
            />
          </label>
          <label>
            容器端口
            <InputNumber
              aria-label={`容器端口 ${index + 1}`}
              min={1}
              max={65535}
              precision={0}
              value={port.containerPort || null}
              onChange={(value) => update(index, { containerPort: value ?? 0 })}
            />
          </label>
          <label>
            主机端口（可选）
            <InputNumber
              aria-label={`主机端口 ${index + 1}`}
              min={0}
              max={65535}
              precision={0}
              placeholder="不映射"
              value={port.hostPort || null}
              onChange={(value) => update(index, { hostPort: value ?? 0 })}
            />
          </label>
          <label>
            主机 IP（可选）
            <Input
              aria-label={`主机 IP ${index + 1}`}
              value={port.hostIP}
              placeholder="默认绑定地址"
              onChange={(event) => update(index, { hostIP: event.target.value })}
            />
          </label>
          <Button
            className="form-item-action"
            onClick={() => {
              setKeys(keys.filter((_, i) => i !== index));
              onChange(ports.filter((_, i) => i !== index));
            }}
          >
            移除此端口
          </Button>
        </div>
      ))}
      <Button
        className="form-item-action"
        onClick={() => {
          setKeys([...keys, crypto.randomUUID()]);
          onChange([
            ...ports,
            {
              originalKey: '',
              name: '',
              protocol: 'TCP',
              containerPort: 8080,
              hostPort: 0,
              hostIP: '',
            },
          ]);
        }}
      >
        添加端口
      </Button>
    </section>
  );
}
