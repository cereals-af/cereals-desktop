import type { CustomValue, Ref, ResourceType, VisualSnapshot } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';
import type { VisualKind } from './resourceSchema';

type Literal = string | number | boolean | null | Literal[] | { [key: string]: Literal };
// Used only for small, application-owned defaults; server values stay in their lossless tree.
export function tree(value: Literal): CustomValue {
  if (value === null) return valueOf('null');
  if (Array.isArray(value)) return { ...valueOf('array'), items: value.map(tree) };
  if (typeof value === 'object')
    return {
      ...valueOf('object'),
      properties: Object.fromEntries(Object.entries(value).map(([key, v]) => [key, tree(v)])),
    };
  return valueOf(typeof value === 'number' ? 'integer' : typeof value, String(value));
}
export function visualKind(
  type: Pick<ResourceType, 'group' | 'version' | 'resource'>,
): VisualKind | undefined {
  if (type.version !== 'v1') return undefined;
  if (type.group === 'batch') {
    if (type.resource === 'jobs') return 'Job';
    if (type.resource === 'cronjobs') return 'CronJob';
  }
  if (type.group === 'networking.k8s.io') {
    if (type.resource === 'ingresses') return 'Ingress';
    if (type.resource === 'networkpolicies') return 'NetworkPolicy';
  }
  if (!type.group && type.resource === 'secrets') return 'Secret';
}
export function newSnapshot(kind: VisualKind, target: Ref): VisualSnapshot {
  const template = { spec: { restartPolicy: 'Never', containers: [{ name: 'app', image: '' }] } };
  const base = { name: '', namespace: target.namespace };
  let value: CustomValue;
  switch (kind) {
    case 'Job':
      value = tree({
        ...base,
        spec: { parallelism: 1, completions: 1, backoffLimit: 6, template },
      });
      break;
    case 'CronJob':
      value = tree({
        ...base,
        spec: {
          schedule: '0 * * * *',
          concurrencyPolicy: 'Forbid',
          suspend: false,
          successfulJobsHistoryLimit: 3,
          failedJobsHistoryLimit: 1,
          jobTemplate: { spec: { backoffLimit: 6, template } },
        },
      });
      break;
    case 'Ingress':
      value = tree({
        ...base,
        spec: {
          rules: [
            {
              host: '',
              http: {
                paths: [
                  {
                    path: '/',
                    pathType: 'Prefix',
                    backend: { service: { name: '', port: { number: 80 } } },
                  },
                ],
              },
            },
          ],
        },
      });
      break;
    case 'NetworkPolicy':
      value = tree({ ...base, spec: { podSelector: {}, policyTypes: ['Ingress'], ingress: [] } });
      break;
    case 'Secret':
      value = tree({ ...base, type: 'Opaque', immutable: false });
      break;
  }
  return {
    baseline: { uid: '', resourceVersion: '' },
    value,
    status: valueOf('null'),
    entries: [],
    revealed: true,
  };
}
export function suspended(value: CustomValue): boolean {
  return property(property(value, 'spec') ?? valueOf('object'), 'suspend')?.text === 'true';
}
export function toggleSuspend(value: CustomValue): CustomValue {
  return setProperty(
    value,
    'spec',
    setProperty(
      property(value, 'spec') ?? valueOf('object'),
      'suspend',
      valueOf('boolean', String(!suspended(value))),
    ),
  );
}
