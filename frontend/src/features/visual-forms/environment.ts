import type { CustomValue } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';

export const environmentSources = [
  { value: 'value', label: '直接赋值' },
  { value: 'secretKeyRef', label: 'Secret 键' },
  { value: 'configMapKeyRef', label: 'ConfigMap 键' },
  { value: 'fieldRef', label: 'Pod 字段' },
  { value: 'resourceFieldRef', label: '容器资源' },
];

export function environmentSource(entry: CustomValue): string {
  const reference = property(entry, 'valueFrom');
  if (!reference) return 'value';
  return environmentSources.find((source) => property(reference, source.value))?.value ?? 'unknown';
}

// Replace only the mutually exclusive source; retain name and unrelated API fields.
export function changeEnvironmentSource(entry: CustomValue, source: string): CustomValue {
  if (source === environmentSource(entry)) return entry;
  const next = setProperty(setProperty(entry, 'value', undefined), 'valueFrom', undefined);
  return source === 'value'
    ? setProperty(next, 'value', valueOf('string'))
    : setProperty(next, 'valueFrom', setProperty(valueOf('object'), source, valueOf('object')));
}

export function updateEnvironmentReference(
  entry: CustomValue,
  source: string,
  reference: CustomValue,
): CustomValue {
  const from = property(entry, 'valueFrom') ?? valueOf('object');
  return setProperty(entry, 'valueFrom', setProperty(from, source, reference));
}
