import { useState } from 'react';
import { Button } from 'antd';
import type { WorkloadContainer } from '../../api/types';
import { ContainerFields } from './ContainerFields';
import { emptyContainer } from './defaults';

export function ContainerList({
  value,
  onChange,
}: {
  value: WorkloadContainer[];
  onChange: (value: WorkloadContainer[]) => void;
}) {
  // Draft names are editable; local keys keep focus and expanded sections on the same row.
  const [keys, setKeys] = useState(() => value.map(() => crypto.randomUUID()));
  const remove = (index: number) => {
    if (value.length <= 1) return;
    setKeys(keys.filter((_, i) => i !== index));
    onChange(value.filter((_, i) => i !== index));
  };
  const add = () => {
    let suffix = value.length + 1;
    while (value.some((container) => container.name === `app-${suffix}`)) suffix++;
    setKeys([...keys, crypto.randomUUID()]);
    onChange([...value, { ...emptyContainer(), name: `app-${suffix}` }]);
  };
  return (
    <>
      {value.map((container, index) => (
        <ContainerFields
          key={keys[index]}
          value={container}
          canRemove={value.length > 1}
          onRemove={() => remove(index)}
          onChange={(next) => onChange(value.map((row, i) => (i === index ? next : row)))}
        />
      ))}
      <div className="mutation-actions">
        <Button className="form-item-action" onClick={add}>
          添加容器
        </Button>
        <span className="muted">至少保留一个容器；修改将在预检并确认提交后生效。</span>
      </div>
    </>
  );
}
