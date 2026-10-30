import { useState } from 'react';
import { Button, Input, Select } from 'antd';
import type { CustomValue } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';
import type { Field } from './schema';

export function ExpressionEditor({
  value,
  onChange,
  path,
  field,
}: {
  value: CustomValue;
  onChange: (value: CustomValue) => void;
  path: string;
  field: Field;
}) {
  const rows = value.items ?? [];
  const [keys, setKeys] = useState(() => rows.map(() => crypto.randomUUID()));
  const update = (index: number, next: CustomValue) =>
    onChange({ ...value, items: rows.map((row, i) => (i === index ? next : row)) });
  const operators = field.item?.fields?.find((item) => item.key === 'operator')?.options ?? [];
  return (
    <div className="visual-collection">
      {rows.map((row, index) => {
        const operator = property(row, 'operator')?.text;
        const noValues = operator === 'Exists' || operator === 'DoesNotExist';
        const rowPath = `${path} ${index + 1}`;
        return (
          <div className="expression-row" key={keys[index]}>
            <label className="visual-field">
              <span>键</span>
              <Input
                aria-label={`${rowPath} / 键`}
                value={property(row, 'key')?.text ?? ''}
                onChange={(event) =>
                  update(index, setProperty(row, 'key', valueOf('string', event.target.value)))
                }
              />
            </label>
            <label className="visual-field">
              <span>运算符</span>
              <Select
                aria-label={`${rowPath} / 运算符`}
                value={operator}
                options={[...new Set([...operators, ...(operator ? [operator] : [])])].map(
                  (item) => ({ value: item, label: item }),
                )}
                onChange={(next) => {
                  let changed = setProperty(row, 'operator', valueOf('string', next));
                  if (next === 'Exists' || next === 'DoesNotExist')
                    changed = setProperty(changed, 'values', undefined);
                  update(index, changed);
                }}
              />
            </label>
            <label className="visual-field">
              <span>匹配值</span>
              <Select
                mode="tags"
                aria-label={`${rowPath} / 匹配值`}
                disabled={noValues}
                placeholder={noValues ? '此运算符无需填写值' : '输入后按 Enter 添加'}
                value={property(row, 'values')?.items?.map((item) => item.text) ?? []}
                onChange={(values: string[]) =>
                  update(
                    index,
                    setProperty(row, 'values', {
                      ...valueOf('array'),
                      items: values.map((item) => valueOf('string', item)),
                    }),
                  )
                }
              />
            </label>
            <Button
              className="form-item-action"
              aria-label={`移除 ${rowPath}`}
              onClick={() => {
                setKeys(keys.filter((_, i) => i !== index));
                onChange({ ...value, items: rows.filter((_, i) => i !== index) });
              }}
            >
              移除条件
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
            items: [...rows, setProperty(valueOf('object'), 'operator', valueOf('string', 'In'))],
          });
        }}
      >
        添加条件
      </Button>
    </div>
  );
}
