import { useState } from 'react';
import { Alert, Button, Collapse, Input, Select } from 'antd';
import type { CustomValue } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';
import { initial, type Field, type FieldsProps } from './schema';
import { EnvironmentEditor } from './EnvironmentEditor';
import { EnvironmentImports } from './EnvironmentImports';
import { AffinityEditor } from './AffinityEditor';
import { ExpressionEditor } from './ExpressionEditor';
import { FieldGroups } from './FieldGroups';
import { containerGroups, podGroups } from './groupDefinitions';

interface Props {
  field: Field;
  value?: CustomValue;
  onChange: (value: CustomValue | undefined) => void;
  path?: string;
  readOnly?: boolean;
  bare?: boolean;
}
export function Fields({ fields, value, onChange, readOnly = false, path = '' }: FieldsProps) {
  return (
    <div className="visual-fields">
      {fields.map((field) => (
        <FieldInput
          key={field.key}
          field={field}
          value={field.key ? property(value, field.key) : value}
          onChange={(next) =>
            onChange(field.key ? setProperty(value, field.key, next) : (next ?? value))
          }
          path={path ? `${path} / ${field.label}` : field.label}
          readOnly={readOnly}
        />
      ))}
    </div>
  );
}
function FieldInput({
  field,
  value,
  onChange,
  path = field.label,
  readOnly = false,
  bare = false,
}: Props) {
  if (readOnly && !value) return null;
  const complex = ['object', 'array', 'map', 'choice'].includes(field.kind);
  if (complex && (bare || (field.kind === 'object' && field.inline)))
    return (
      <div className="visual-field-group">
        <ComplexField
          field={field}
          value={value ?? initial(field)}
          onChange={onChange}
          path={path}
          readOnly={readOnly}
        />
      </div>
    );
  const directlyEditable =
    !readOnly &&
    (field.direct ||
      field.inline ||
      field.editor ||
      (field.kind === 'array' && field.item?.kind === 'string'));
  if (complex)
    return (
      <section className="visual-section">
        <div className="configuration-section-heading">
          <div className="container-section-heading">
            <h4>{field.label}</h4>
            {field.help && <p className="muted">{field.help}</p>}
          </div>
          {value && !field.required && !readOnly && (
            <Button
              type="text"
              className="form-item-action"
              aria-label={`清除 ${path}`}
              onClick={() => onChange(undefined)}
            >
              清除此配置
            </Button>
          )}
        </div>
        {!value && !field.required && !directlyEditable ? (
          <div className="visual-optional">
            <span className="muted">未配置</span>
            {!readOnly && (
              <Button
                className="form-item-action"
                aria-label={`配置 ${path}`}
                onClick={() => onChange(initial(field))}
              >
                配置选项
              </Button>
            )}
          </div>
        ) : (
          <>
            <ComplexField
              key={value ? 'configured' : 'empty'}
              field={field}
              value={value ?? initial(field)}
              onChange={onChange}
              path={path}
              readOnly={readOnly}
            />
          </>
        )}
      </section>
    );
  if (readOnly)
    return (
      <div className="visual-field">
        <span>{field.label}</span>
        <code className="config-preview visual-readonly-value">
          {field.kind === 'boolean'
            ? value?.text === 'true'
              ? '是'
              : '否'
            : value?.text || '空字符串'}
        </code>
      </div>
    );
  const change = (text: string) =>
    onChange(
      valueOf(
        field.kind === 'port' ? (/^\d+$/.test(text) ? 'integer' : 'string') : field.kind,
        text,
      ),
    );
  return (
    <label className="visual-field">
      <span>{field.label}</span>
      {field.kind === 'boolean' ? (
        <Select
          aria-label={path}
          disabled={readOnly}
          allowClear
          value={value?.text}
          placeholder="未设置（集群默认）"
          options={[
            { value: 'true', label: '是' },
            { value: 'false', label: '否' },
          ]}
          onChange={(v) => (v === undefined ? onChange(undefined) : change(v))}
        />
      ) : field.options ? (
        <Select
          aria-label={path}
          disabled={readOnly}
          allowClear
          value={value?.text}
          placeholder="未设置（集群默认）"
          options={[...new Set([...field.options, ...(value?.text ? [value.text] : [])])].map(
            (v) => ({ value: v, label: v }),
          )}
          onChange={(v) => (v === undefined ? onChange(undefined) : change(v))}
        />
      ) : (
        <Input.TextArea
          aria-label={path}
          readOnly={readOnly}
          autoSize={{ minRows: 1, maxRows: 6 }}
          value={value?.text ?? ''}
          placeholder={field.kind === 'integer' ? '未设置（整数）' : undefined}
          onChange={(e) =>
            field.kind === 'integer' && e.target.value === ''
              ? onChange(undefined)
              : change(e.target.value)
          }
        />
      )}
      {field.help && <span className="muted">{field.help}</span>}
    </label>
  );
}
function ComplexField(props: Props & { value: CustomValue }) {
  const { field, value, onChange, path = props.field.label, readOnly } = props;
  const renderFields = (next: FieldsProps) => <Fields {...next} />;
  if (!readOnly) {
    const editorProps = {
      field,
      value,
      path,
      onChange: (next: CustomValue) => onChange(next),
      renderFields,
    };
    if (field.editor === 'environment') return <EnvironmentEditor {...editorProps} />;
    if (field.editor === 'environmentImports') return <EnvironmentImports {...editorProps} />;
    if (field.editor === 'affinity') return <AffinityEditor {...editorProps} />;
    if (field.editor === 'expressions') return <ExpressionEditor {...editorProps} />;
    if (field.editor === 'container' || field.editor === 'pod') {
      const identity = field.editor === 'container' ? ['name', 'image'] : ['containers'];
      const fields = field.fields ?? [];
      return (
        <>
          <Fields
            fields={fields.filter((item) => identity.includes(item.key))}
            value={value}
            onChange={onChange}
            path={path}
          />
          <FieldGroups
            fields={fields.filter((item) => !identity.includes(item.key))}
            value={value}
            onChange={onChange}
            path={path}
            groups={field.editor === 'container' ? containerGroups : podGroups}
            renderFields={renderFields}
          />
        </>
      );
    }
  }
  if (field.kind === 'array') return <ArrayInput {...props} />;
  if (field.kind === 'map') return <MapInput {...props} />;
  if (field.kind === 'choice') {
    const selected = field.fields?.find((f) => property(value, f.key));
    return (
      <>
        <Select
          aria-label={`${path} 类型`}
          disabled={readOnly}
          allowClear
          placeholder="选择类型"
          value={selected?.key}
          options={field.fields?.map((f) => ({ value: f.key, label: f.label }))}
          onChange={(key) => {
            let next = value;
            for (const f of field.fields ?? []) next = setProperty(next, f.key, undefined);
            const f = field.fields?.find((f) => f.key === key);
            onChange(f ? setProperty(next, f.key, initial(f)) : next);
          }}
        />
        {selected && (
          <FieldInput
            field={{ ...selected, required: true }}
            value={property(value, selected.key)}
            onChange={(next) => onChange(setProperty(value, selected.key, next))}
            path={`${path} / ${selected.label}`}
            readOnly={readOnly}
            bare={selected.kind === 'object' || selected.kind === 'choice'}
          />
        )}
      </>
    );
  }
  return (
    <Fields
      fields={field.fields ?? []}
      value={value}
      onChange={onChange}
      path={path}
      readOnly={readOnly}
    />
  );
}
function ArrayInput({ field, value, onChange, path, readOnly }: Props & { value: CustomValue }) {
  const items = value.items ?? [];
  const [keys, setKeys] = useState(() => items.map(() => crypto.randomUUID()));
  const [active, setActive] = useState<string[]>(keys.slice(0, 1));
  const item = field.item;
  if (!item) return null;
  const inline = field.inline || !['object', 'array', 'map', 'choice'].includes(item.kind);
  const rows = items.map((entry, index) => ({
    key: keys[index] ?? String(index),
    label: `${field.label} ${index + 1}${property(entry, 'name')?.text ? ` · ${property(entry, 'name')?.text}` : ''}`,
    children: (
      <div className={`visual-array-content${inline ? ' visual-array-inline' : ''}`}>
        <FieldInput
          field={{ ...item, required: true }}
          value={entry}
          bare={item.kind === 'object' || item.kind === 'choice'}
          path={`${path} ${index + 1}`}
          readOnly={readOnly}
          onChange={(next) => {
            if (next)
              onChange({ ...value, items: items.map((row, i) => (i === index ? next : row)) });
          }}
        />
        {!readOnly && (
          <Button
            className="form-item-action"
            aria-label={`移除 ${path} ${index + 1}`}
            onClick={() => {
              setKeys(keys.filter((_, i) => i !== index));
              onChange({ ...value, items: items.filter((_, i) => i !== index) });
            }}
          >
            移除此项
          </Button>
        )}
      </div>
    ),
  }));
  return (
    <div className="visual-collection">
      {inline ? (
        rows.map((row) => (
          <div
            key={row.key}
            className={
              field.editor === 'ports'
                ? 'configuration-row generic-port-row'
                : item.kind === 'object'
                  ? 'configuration-row'
                  : undefined
            }
          >
            {item.kind === 'object' && field.editor !== 'ports' && <h4>{row.label}</h4>}
            {row.children}
          </div>
        ))
      ) : (
        <Collapse
          activeKey={active}
          onChange={(key) => setActive(typeof key === 'string' ? [key] : key)}
          items={rows}
        />
      )}
      {!readOnly && (
        <Button
          className="form-item-action"
          aria-label={`添加 ${path}`}
          onClick={() => {
            const id = crypto.randomUUID();
            setKeys([...keys, id]);
            setActive([...active, id]);
            onChange({ ...value, items: [...items, initial(item)] });
          }}
        >
          添加一项
        </Button>
      )}
    </div>
  );
}
function MapInput({ value, onChange, path, readOnly }: Props & { value: CustomValue }) {
  const entries = Object.entries(value.properties ?? {}).filter(
    (entry): entry is [string, CustomValue] => entry[1] !== undefined,
  );
  const [error, setError] = useState('');
  return (
    <div className="visual-collection">
      {error && <Alert type="error" title={error} />}
      {entries.map(([key, entry], index) => (
        <div className="environment-row" key={key}>
          <Input
            aria-label={`${path} 键 ${index + 1}`}
            readOnly={readOnly}
            defaultValue={key}
            onBlur={(e) => {
              const name = e.target.value;
              if (name === key) return;
              if (!name || Object.hasOwn(value.properties ?? {}, name)) {
                setError('键名不能为空或重复');
                e.target.value = key;
                return;
              }
              setError('');
              onChange(setProperty(setProperty(value, key, undefined), name, entry));
            }}
          />
          <Input.TextArea
            aria-label={`${path} 值 ${index + 1}`}
            readOnly={readOnly}
            autoSize={{ minRows: 1, maxRows: 6 }}
            value={entry.text}
            onChange={(e) => onChange(setProperty(value, key, valueOf('string', e.target.value)))}
          />
          {!readOnly && (
            <Button
              className="form-item-action"
              onClick={() => onChange(setProperty(value, key, undefined))}
            >
              移除
            </Button>
          )}
        </div>
      ))}
      {!readOnly && (
        <Button
          className="form-item-action"
          aria-label={`添加 ${path}`}
          onClick={() => {
            let n = entries.length + 1;
            while (Object.hasOwn(value.properties ?? {}, `key-${n}`)) n++;
            onChange(setProperty(value, `key-${n}`, valueOf('string')));
          }}
        >
          添加键值
        </Button>
      )}
    </div>
  );
}
