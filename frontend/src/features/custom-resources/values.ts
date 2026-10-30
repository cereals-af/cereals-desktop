import type { CustomField, CustomValue } from '../../api/types';

export function valueOf(type: string, text = ''): CustomValue {
  return { type, text, properties: {}, items: [] };
}
export function initialValue(field: CustomField): CustomValue {
  return valueOf(
    field.type,
    field.enum?.[0] ??
      (field.type === 'boolean'
        ? 'false'
        : field.type === 'integer' || field.type === 'number'
          ? '0'
          : ''),
  );
}
export function property(value: CustomValue, key: string): CustomValue | undefined {
  return value.properties && Object.hasOwn(value.properties, key)
    ? value.properties[key]
    : undefined;
}
export function setProperty(
  value: CustomValue,
  key: string,
  next: CustomValue | undefined,
): CustomValue {
  if (next) return { ...value, properties: { ...value.properties, [key]: next } };
  const properties = { ...value.properties };
  delete properties[key];
  return { ...value, properties };
}
export function identity(value: CustomValue, name: string): string {
  const metadata = property(value, 'metadata');
  return metadata ? (property(metadata, name)?.text ?? '') : '';
}
export function setIdentity(value: CustomValue, name: string, text: string): CustomValue {
  return setProperty(
    value,
    'metadata',
    setProperty(property(value, 'metadata') ?? valueOf('object'), name, valueOf('string', text)),
  );
}
