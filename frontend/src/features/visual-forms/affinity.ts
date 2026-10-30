import type { CustomValue } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';

export const affinityTypes = [
  { value: 'nodeAffinity', label: '节点亲和性' },
  { value: 'podAffinity', label: 'Pod 亲和性' },
  { value: 'podAntiAffinity', label: 'Pod 反亲和性' },
];
export const requiredAffinity = 'requiredDuringSchedulingIgnoredDuringExecution';
export const preferredAffinity = 'preferredDuringSchedulingIgnoredDuringExecution';

export function affinityRules(
  value: CustomValue,
  type: string,
  mode: string,
): CustomValue | undefined {
  const group = property(value, type);
  const rules = group && property(group, mode);
  return rules && type === 'nodeAffinity' && mode === requiredAffinity
    ? property(rules, 'nodeSelectorTerms')
    : rules;
}

// UI sections are flat; edits still target the original Kubernetes object paths.
export function setAffinityRules(
  value: CustomValue,
  type: string,
  mode: string,
  rules: CustomValue | undefined,
): CustomValue {
  const group = property(value, type) ?? valueOf('object');
  let next = rules;
  if (type === 'nodeAffinity' && mode === requiredAffinity) {
    const required = setProperty(
      property(group, mode) ?? valueOf('object'),
      'nodeSelectorTerms',
      rules,
    );
    next = rules || Object.keys(required.properties ?? {}).length ? required : undefined;
  }
  const updated = setProperty(group, mode, next);
  return setProperty(
    value,
    type,
    Object.keys(updated.properties ?? {}).length ? updated : undefined,
  );
}
