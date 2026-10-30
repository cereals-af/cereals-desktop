import { useState, type ReactNode } from 'react';
import { Button, Input, Select } from 'antd';
import type { CustomValue } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';
import {
  changeEnvironmentSource,
  environmentSource,
  environmentSources,
  updateEnvironmentReference,
} from './environment';
import type { Field, FieldsProps } from './schema';

export function EnvironmentEditor({
  value,
  onChange,
  path,
  field,
  renderFields,
}: {
  value: CustomValue;
  onChange: (value: CustomValue) => void;
  path: string;
  field: Field;
  renderFields: (props: FieldsProps) => ReactNode;
}) {
  const entries = value.items ?? [];
  const [keys, setKeys] = useState(() => entries.map(() => crypto.randomUUID()));
  const update = (index: number, next: CustomValue) =>
    onChange({ ...value, items: entries.map((entry, i) => (i === index ? next : entry)) });
  const references =
    field.item?.fields
      ?.find((item) => item.kind === 'choice')
      ?.fields?.find((item) => item.key === 'valueFrom')?.fields ?? [];
  return (
    <div className="visual-collection">
      {!entries.length && <p className="muted">未设置环境变量，使用镜像默认配置。</p>}
      {entries.map((entry, index) => {
        const source = environmentSource(entry);
        const from = property(entry, 'valueFrom');
        const reference = from ? property(from, source) : undefined;
        const rowPath = `${path} ${index + 1}`;
        return (
          <div
            className="configuration-row environment-entry"
            data-source={source}
            key={keys[index]}
          >
            <div className="environment-entry-fields">
              <label className="visual-field">
                <span>变量名称</span>
                <Input
                  aria-label={`${rowPath} / 变量名称`}
                  value={property(entry, 'name')?.text ?? ''}
                  onChange={(event) =>
                    update(index, setProperty(entry, 'name', valueOf('string', event.target.value)))
                  }
                />
              </label>
              <label className="visual-field">
                <span>变量来源</span>
                <Select
                  aria-label={`${rowPath} / 变量来源`}
                  value={source}
                  options={
                    source === 'unknown'
                      ? [
                          ...environmentSources,
                          { value: 'unknown', label: '其他来源（保留）', disabled: true },
                        ]
                      : environmentSources
                  }
                  onChange={(next) => update(index, changeEnvironmentSource(entry, next))}
                />
              </label>
              {source === 'value' && (
                <label className="visual-field environment-entry-value">
                  <span>变量值</span>
                  <Input.TextArea
                    aria-label={`${rowPath} / 变量值`}
                    autoSize={{ minRows: 1, maxRows: 6 }}
                    value={property(entry, 'value')?.text ?? ''}
                    onChange={(event) =>
                      update(
                        index,
                        setProperty(entry, 'value', valueOf('string', event.target.value)),
                      )
                    }
                  />
                </label>
              )}
            </div>
            {reference &&
              renderFields({
                fields: references.find((item) => item.key === source)?.fields ?? [],
                value: reference,
                path: rowPath,
                onChange: (next) => update(index, updateEnvironmentReference(entry, source, next)),
              })}
            <Button
              className="form-item-action"
              aria-label={`移除 ${rowPath}`}
              onClick={() => {
                setKeys(keys.filter((_, i) => i !== index));
                onChange({ ...value, items: entries.filter((_, i) => i !== index) });
              }}
            >
              移除变量
            </Button>
            {source === 'unknown' && (
              <p className="muted">当前来源保留；可在 YAML 中查看，或选择新的变量来源。</p>
            )}
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
            items: [
              ...entries,
              setProperty(
                setProperty(valueOf('object'), 'name', valueOf('string')),
                'value',
                valueOf('string'),
              ),
            ],
          });
        }}
      >
        添加环境变量
      </Button>
    </div>
  );
}
