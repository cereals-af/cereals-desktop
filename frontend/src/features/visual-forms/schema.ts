import type { CustomValue } from '../../api/types';
import { valueOf } from '../custom-resources/values';

export interface Field {
  key: string;
  label: string;
  kind: 'string' | 'integer' | 'boolean' | 'object' | 'array' | 'map' | 'choice' | 'port';
  help?: string;
  required?: boolean;
  options?: string[];
  fields?: Field[];
  item?: Field;
  initial?: CustomValue;
  editor?:
    | 'environment'
    | 'environmentImports'
    | 'affinity'
    | 'container'
    | 'pod'
    | 'expressions'
    | 'ports';
  inline?: boolean;
  direct?: boolean;
}

export interface FieldsProps {
  fields: Field[];
  value: CustomValue;
  onChange: (value: CustomValue) => void;
  readOnly?: boolean;
  path?: string;
}
export const text = (key: string, label: string, options?: string[]): Field => ({
  key,
  label,
  kind: 'string',
  options,
});
export const number = (key: string, label: string): Field => ({ key, label, kind: 'integer' });
export const bool = (key: string, label: string): Field => ({ key, label, kind: 'boolean' });
export const object = (key: string, label: string, fields: Field[], required = false): Field => ({
  key,
  label,
  kind: 'object',
  fields,
  required,
});
export const list = (key: string, label: string, item: Field): Field => ({
  key,
  label,
  kind: 'array',
  item,
});
export const map = (key: string, label: string): Field => ({ key, label, kind: 'map' });
export const choice = (key: string, label: string, fields: Field[]): Field => ({
  key,
  label,
  kind: 'choice',
  fields,
});
export const strings = (key: string, label: string) => list(key, label, text('', '值'));
export const port = (key: string, label: string): Field => ({
  key,
  label,
  kind: 'port',
  help: '填写端口号或命名端口；数字会作为整数提交。',
});
export function initial(field: Field): CustomValue {
  if (field.initial) return structuredClone(field.initial);
  if (field.kind === 'object' || field.kind === 'map' || field.kind === 'choice') {
    const value = valueOf('object');
    for (const child of field.fields ?? [])
      if (child.required || child.initial)
        value.properties = { ...value.properties, [child.key]: initial(child) };
    return value;
  }
  return valueOf(
    field.kind === 'port' ? 'integer' : field.kind,
    field.options?.[0] ??
      (field.kind === 'boolean'
        ? 'false'
        : field.kind === 'integer' || field.kind === 'port'
          ? '0'
          : ''),
  );
}
