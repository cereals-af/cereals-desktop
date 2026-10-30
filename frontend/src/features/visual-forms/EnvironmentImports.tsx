import { useState, type ReactNode } from 'react';
import { Button, Input, Select } from 'antd';
import type { CustomValue } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';
import type { Field, FieldsProps } from './schema';

export function EnvironmentImports({
  field,
  value,
  onChange,
  path,
  renderFields,
}: {
  field: Field;
  value: CustomValue;
  onChange: (value: CustomValue) => void;
  path: string;
  renderFields: (props: FieldsProps) => ReactNode;
}) {
  const entries = value.items ?? [];
  const [keys, setKeys] = useState(() => entries.map(() => crypto.randomUUID()));
  const sources = field.item?.fields?.find((item) => item.kind === 'choice')?.fields ?? [];
  const update = (index: number, next: CustomValue) =>
    onChange({ ...value, items: entries.map((entry, i) => (i === index ? next : entry)) });
  return (
    <div className="visual-collection">
      <p className="muted">将 ConfigMap 或 Secret 的全部键导入为环境变量，可添加统一前缀。</p>
      {entries.map((entry, index) => {
        const source = sources.find((item) => property(entry, item.key));
        const reference = source && property(entry, source.key);
        const rowPath = `${path} ${index + 1}`;
        return (
          <div className="configuration-row environment-entry environment-import" key={keys[index]}>
            <label className="visual-field">
              <span>变量前缀（可选）</span>
              <Input
                aria-label={`${rowPath} / 变量前缀`}
                value={property(entry, 'prefix')?.text ?? ''}
                onChange={(event) =>
                  update(index, setProperty(entry, 'prefix', valueOf('string', event.target.value)))
                }
              />
            </label>
            <label className="visual-field">
              <span>引用来源</span>
              <Select
                aria-label={`${rowPath} / 引用来源`}
                value={source?.key}
                placeholder="选择来源"
                options={sources.map((item) => ({ value: item.key, label: item.label }))}
                onChange={(key) => {
                  if (source?.key === key) return;
                  let next = entry;
                  for (const item of sources) next = setProperty(next, item.key, undefined);
                  update(index, setProperty(next, key, valueOf('object')));
                }}
              />
            </label>
            {source &&
              reference &&
              renderFields({
                fields: source.fields ?? [],
                value: reference,
                path: rowPath,
                onChange: (next) => update(index, setProperty(entry, source.key, next)),
              })}
            <Button
              className="form-item-action"
              aria-label={`移除 ${rowPath}`}
              onClick={() => {
                setKeys(keys.filter((_, i) => i !== index));
                onChange({ ...value, items: entries.filter((_, i) => i !== index) });
              }}
            >
              移除引用
            </Button>
          </div>
        );
      })}
      <Button
        className="form-item-action"
        aria-label={`添加 ${path}`}
        onClick={() => {
          setKeys([...keys, crypto.randomUUID()]);
          onChange({
            ...value,
            items: [...entries, setProperty(valueOf('object'), 'configMapRef', valueOf('object'))],
          });
        }}
      >
        添加批量引用
      </Button>
    </div>
  );
}
