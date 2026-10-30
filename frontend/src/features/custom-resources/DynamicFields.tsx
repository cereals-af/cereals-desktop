import { Alert, Button, Input, Select, Space, Switch, Tag } from 'antd';
import type { CustomField, CustomValue } from '../../api/types';
import { initialValue, property, setProperty, valueOf } from './values';

interface Props {
  field: CustomField;
  value?: CustomValue;
  path: string;
  onChange: (value: CustomValue | undefined) => void;
}
export function DynamicFields({
  fields,
  value,
  onChange,
}: {
  fields: CustomField[];
  value: CustomValue;
  onChange: (value: CustomValue) => void;
}) {
  return (
    <div className="dynamic-fields">
      {fields.map((field) => (
        <DynamicField
          key={field.name}
          field={field}
          value={property(value, field.name)}
          path={field.name}
          onChange={(next) => onChange(setProperty(value, field.name, next))}
        />
      ))}
    </div>
  );
}
function DynamicField({ field, value, path, onChange }: Props) {
  const mismatch =
    value &&
    value.type !== 'null' &&
    value.type !== field.type &&
    !(field.type === 'number' && value.type === 'integer');
  return (
    <section className="dynamic-field">
      <div className="dynamic-field-heading">
        <strong>{field.name}</strong>
        <Space wrap>
          <Tag>{field.type || 'unknown'}</Tag>
          {field.required && <Tag color="orange">必填</Tag>}
          {!field.reason && value && !field.required && (
            <Button size="small" aria-label={`移除 ${path}`} onClick={() => onChange(undefined)}>
              移除
            </Button>
          )}
          {!field.reason && field.nullable && value?.type !== 'null' && (
            <Button
              size="small"
              aria-label={`设为 null ${path}`}
              onClick={() => onChange(valueOf('null'))}
            >
              设为 null
            </Button>
          )}
        </Space>
      </div>
      {field.description && <p className="muted">{field.description}</p>}
      {field.reason ? (
        <p className="muted">{field.reason}。已有内容会保留。</p>
      ) : mismatch ? (
        <Alert type="warning" title="当前值与 Schema 类型不一致，已保留；请使用 YAML 检查" />
      ) : !value || value.type === 'null' ? (
        <Space>
          <span className="muted">{value ? 'null' : '未设置'}</span>
          <Button
            size="small"
            aria-label={`设置 ${path}`}
            onClick={() => onChange(initialValue(field))}
          >
            设置字段
          </Button>
        </Space>
      ) : (
        <FieldValue field={field} value={value} path={path} onChange={onChange} />
      )}
    </section>
  );
}
function FieldValue({ field, value, path, onChange }: Props & { value: CustomValue }) {
  if (field.type === 'object') {
    const known = new Set(field.fields?.map((child) => child.name));
    const unknown = Object.keys(value.properties ?? {}).filter((name) => !known.has(name));
    return (
      <>
        {unknown.length > 0 && (
          <p className="muted">保留 {unknown.length} 个未展示字段，使用 YAML 查看或编辑。</p>
        )}
        <div className="dynamic-children">
          {field.fields?.map((child) => (
            <DynamicField
              key={child.name}
              field={child}
              value={property(value, child.name)}
              path={`${path}.${child.name}`}
              onChange={(next) => onChange(setProperty(value, child.name, next))}
            />
          ))}
        </div>
      </>
    );
  }
  if (field.type === 'array')
    return <ArrayFields field={field} value={value} path={path} onChange={onChange} />;
  const change = (text: string) => onChange(valueOf(field.type, text));
  if (field.enum?.length)
    return (
      <Select
        aria-label={path}
        value={value.text}
        onChange={change}
        options={field.enum.map((text) => ({ value: text, label: text }))}
      />
    );
  if (field.type === 'boolean')
    return (
      <Switch
        aria-label={path}
        checked={value.text === 'true'}
        onChange={(checked) => change(String(checked))}
      />
    );
  return (
    <Input.TextArea
      aria-label={path}
      autoSize={{ minRows: 1, maxRows: 6 }}
      value={value.text}
      onChange={(event) => change(event.target.value)}
      placeholder={field.type === 'integer' ? '整数（文本保存，支持 int64）' : undefined}
    />
  );
}
function ArrayFields({ field, value, path, onChange }: Props & { value: CustomValue }) {
  const itemSchema = field.items;
  if (!itemSchema) return <p className="muted">缺少元素 Schema，请使用 YAML。</p>;
  const items = value.items ?? [];
  return (
    <div className="dynamic-array">
      {items.slice(0, 200).map((item, index) => (
        <div className="dynamic-array-item" key={index}>
          <DynamicField
            field={{ ...itemSchema, name: `第 ${index + 1} 项`, required: true }}
            value={item}
            path={`${path}[${index}]`}
            onChange={(next) => {
              if (next)
                onChange({ ...value, items: items.map((row, i) => (i === index ? next : row)) });
            }}
          />
          <Button
            aria-label={`删除 ${path}[${index}]`}
            onClick={() => onChange({ ...value, items: items.filter((_, i) => i !== index) })}
          >
            删除此项
          </Button>
        </div>
      ))}
      {items.length > 200 && (
        <Alert type="info" title="仅展示前 200 项，其余项目保留；请使用 YAML 编辑更大的数组" />
      )}
      <Button
        disabled={items.length >= 200 || Boolean(itemSchema.reason)}
        aria-label={`添加 ${path}`}
        onClick={() => onChange({ ...value, items: [...items, initialValue(itemSchema)] })}
      >
        添加数组项
      </Button>
    </div>
  );
}
