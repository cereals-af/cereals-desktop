import { useState } from 'react';
import { Button, Input } from 'antd';
import type { ControllerForm } from '../../api/types';

export function NodeSelectorFields({
  value,
  onChange,
}: {
  value: NonNullable<ControllerForm['nodeSelector']>;
  onChange: (value: NonNullable<ControllerForm['nodeSelector']>) => void;
}) {
  const [keys, setKeys] = useState(() => value.map(() => crypto.randomUUID()));
  return (
    <section className="visual-section">
      <div className="container-section-heading">
        <h4>节点选择器</h4>
        <p className="muted">所有节点标签需同时匹配；更灵活的规则可在下方亲和性中设置。</p>
      </div>
      {value.map((label, index) => (
        <div className="environment-row" key={keys[index]}>
          <Input
            aria-label={`节点标签键 ${index + 1}`}
            placeholder="标签键"
            value={label.key}
            onChange={(event) =>
              onChange(
                value.map((row, i) => (i === index ? { ...row, key: event.target.value } : row)),
              )
            }
          />
          <Input
            aria-label={`节点标签值 ${index + 1}`}
            placeholder="标签值"
            value={label.value}
            onChange={(event) =>
              onChange(
                value.map((row, i) => (i === index ? { ...row, value: event.target.value } : row)),
              )
            }
          />
          <Button
            className="form-item-action"
            onClick={() => {
              setKeys(keys.filter((_, i) => i !== index));
              onChange(value.filter((_, i) => i !== index));
            }}
          >
            移除标签
          </Button>
        </div>
      ))}
      <Button
        className="form-item-action"
        onClick={() => {
          setKeys([...keys, crypto.randomUUID()]);
          onChange([...value, { key: '', value: '' }]);
        }}
      >
        添加节点标签
      </Button>
    </section>
  );
}
