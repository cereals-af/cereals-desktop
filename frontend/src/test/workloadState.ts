import { tree } from '../features/visual-forms/models';
import type { Ref, WorkloadSnapshot } from '../api/types';
import { emptyContainer } from '../features/workloads/defaults';

export const saved = new Map<string, WorkloadSnapshot>();
export const key = (target: Ref) => `${target.clusterID}/${target.namespace}/${target.name}`;

export function current(target: Ref): WorkloadSnapshot {
  return structuredClone(
    saved.get(key(target)) ?? {
      form: {
        podOptions: null,
        name: target.name,
        namespace: target.namespace,
        replicas: 3,
        containers: [
          {
            ...emptyContainer(),
            originalName: 'app',
            advanced: tree({
              env: [
                {
                  name: 'TOKEN',
                  valueFrom: { secretKeyRef: { name: 'credentials', key: 'token' } },
                },
              ],
            }),
            image: 'nginx:1.29',
            environment: [{ name: 'TOKEN', value: '', source: 'valueFrom（可视化编辑引用）' }],
          },
        ],
      },
      baseline: { uid: key(target), resourceVersion: '100' },
      status: {
        phase: '已就绪',
        strategy: 'RollingUpdate',
        paused: false,
        observed: true,
        replicas: 3,
        updated: 3,
        ready: 3,
        available: 3,
        conditions: [
          {
            type: 'Available',
            status: 'True',
            reason: 'MinimumReplicasAvailable',
            message: 'Deployment has minimum availability.',
          },
        ],
      },
    },
  );
}
