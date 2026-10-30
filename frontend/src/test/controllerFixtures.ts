import type { ControllerRequest, ControllerSnapshot, DesktopAPI, Ref } from '../api/types';
import { emptyContainer } from '../features/workloads/defaults';
import { emptyController } from '../features/workloads/controllers';
import { mutationFixtures } from './mutationFixtures';
import { containerSnapshot } from './containerFixtures';

const saved = new Map<string, ControllerSnapshot>();
const pending = new Map<string, ControllerRequest>();
const key = (target: Ref) =>
  [
    target.clusterID,
    target.group,
    target.version,
    target.resource,
    target.namespace,
    target.name,
  ].join('|');
function current(target: Ref): ControllerSnapshot {
  const form = emptyController(target.namespace);
  form.workload.name = target.name;
  form.workload.replicas = 3;
  form.workload.containers = [
    {
      ...emptyContainer(),
      originalName: 'app',
      image: 'nginx:1.29',
      cpuRequest: '100m',
      cpuLimit: '1',
      memoryRequest: '128Mi',
      memoryLimit: '512Mi',
    },
  ];
  form.serviceName = 'cache-headless';
  form.retainScaled = 'Delete';
  form.nodeSelector = [{ key: 'kubernetes.io/os', value: 'linux' }];
  form.claims =
    target.resource === 'statefulsets'
      ? [
          {
            name: 'data',
            capacity: '10Gi',
            classMode: 'named',
            className: 'fast',
            accessModes: ['ReadWriteOnce'],
            volumeMode: 'Filesystem',
            mountPath: '/data',
          },
        ]
      : [];
  if (target.name === 'ondelete-agent') form.strategy = 'OnDelete';
  return structuredClone(
    saved.get(key(target)) ?? {
      form,
      baseline: { uid: key(target), resourceVersion: '100' },
      owner:
        target.name === 'managed-rs'
          ? { ...target, resource: 'deployments', name: 'api-gateway' }
          : null,
      ownerUID: `${target.clusterID}/${target.namespace}/api-gateway`,
      status: {
        phase: '已就绪',
        observed: true,
        desired: 3,
        current: 3,
        ready: 3,
        updated: 3,
        available: 3,
        unavailable: 0,
        misscheduled: 0,
        currentRevision: 'revision-2',
        updateRevision: 'revision-2',
        conditions: [],
        scheduling: '{"nodeSelector":{"kubernetes.io/os":"linux"}}',
      },
    },
  );
}
export const controllerFixtures: Pick<
  DesktopAPI,
  'controller' | 'previewController' | 'controllerHistory' | 'previewControllerRollback'
> = {
  controller: (target) => Promise.resolve(current(target)),
  previewController: async (request) => {
    if (request.target.namespace !== request.form.workload.namespace)
      throw new Error('命名空间范围不一致');
    if (!request.form.workload.name || !request.form.workload.containers?.every((c) => c.image))
      throw new Error('请填写名称和镜像');
    const preview = await mutationFixtures.preview({
      target: {
        ...request.target,
        name: request.form.workload.name,
        namespace: request.form.workload.namespace,
      },
      mode: request.action === 'create' ? 'create' : 'patch',
      baseline: request.baseline,
      revealSecret: false,
      yaml: JSON.stringify(
        {
          apiVersion: 'apps/v1',
          kind: request.target.resource,
          spec: { action: request.action, form: request.form },
        },
        null,
        2,
      ),
    });
    pending.set(preview.token, structuredClone(request));
    return preview;
  },
  controllerHistory: (target) =>
    Promise.resolve({
      baseline: current(target).baseline,
      paused: false,
      currentRevision: 'revision-2',
      warnings: [],
      truncated: false,
      revisions: [
        {
          name: 'revision-2',
          revision: '2',
          baseline: { uid: 'revision-2-uid', resourceVersion: '2' },
          createdAt: '2026-10-27T02:00:00Z',
          images: ['app: nginx:1.29'],
          replicas: 0,
          ready: 0,
          current: true,
          unavailable: '',
        },
        {
          name: 'revision-1',
          revision: '1',
          baseline: { uid: 'revision-1-uid', resourceVersion: '1' },
          createdAt: '2026-10-26T02:00:00Z',
          images: ['app: nginx:1.28'],
          replicas: 0,
          ready: 0,
          current: false,
          unavailable: '',
        },
        {
          name: 'unknown-revision',
          revision: '0',
          baseline: { uid: 'unknown-uid', resourceVersion: '1' },
          createdAt: '',
          images: [],
          replicas: 0,
          ready: 0,
          current: false,
          unavailable: '不支持的 ControllerRevision 数据格式',
        },
      ],
    }),
  previewControllerRollback: async (request) => {
    const snapshot = current(request.target);
    const container = snapshot.form.workload.containers?.[0];
    if (container) container.image = 'nginx:1.28';
    return controllerFixtures.previewController({
      target: request.target,
      baseline: request.baseline,
      form: snapshot.form,
      action: 'update',
    });
  },
};
export function commitController(token: string) {
  const request = pending.get(token);
  if (!request) return;
  const target = {
    ...request.target,
    name: request.form.workload.name,
    namespace: request.form.workload.namespace,
  };
  const snapshot = current(target);
  snapshot.form = request.form;
  snapshot.form.workload.containers = containerSnapshot(snapshot.form.workload.containers);
  snapshot.baseline.resourceVersion = String(Number(snapshot.baseline.resourceVersion) + 1);
  saved.set(key(target), snapshot);
  pending.delete(token);
}
export function discardController(token: string) {
  pending.delete(token);
}
