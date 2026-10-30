import type {
  DesktopAPI,
  Ref,
  StorageClaim,
  StorageClass,
  Query,
  Page,
  RelationReport,
} from '../api/types';
import { mutationFixtures } from './mutationFixtures';

const claims = new Map<string, StorageClaim>();
const pending = new Map<string, () => void>();
const key = (ref: Ref) => `${ref.clusterID}/${ref.namespace}/${ref.name}`;
function current(ref: Ref): StorageClaim {
  return structuredClone(
    claims.get(key(ref)) ?? {
      baseline: { uid: key(ref), resourceVersion: '100' },
      phase: ref.name === 'waiting-data' ? 'Pending' : 'Bound',
      requested: '10Gi',
      capacity: ref.name === 'waiting-data' ? '' : '10Gi',
      allocated: '',
      resizeStatus: '',
      accessModes: ['ReadWriteOnce'],
      volumeMode: 'Filesystem',
      volumeName: ref.name === 'waiting-data' ? '' : 'data-volume',
      className: 'fast-csi',
      classMode: 'named',
      conditions: [],
      deleting: false,
    },
  );
}
export function commitStorage(token: string) {
  pending.get(token)?.();
  pending.delete(token);
}
export function discardStorage(token: string) {
  pending.delete(token);
}
function classValue(): StorageClass {
  return {
    provisioner: 'example.csi.test',
    reclaimPolicy: 'Delete',
    bindingMode: 'WaitForFirstConsumer',
    allowExpansion: true,
    default: true,
    mountOptions: ['discard'],
    allowedTopologies: '',
  };
}
export const storageFixtures: Pick<
  DesktopAPI,
  'claim' | 'volume' | 'storageClass' | 'previewClaim' | 'previewExpansion'
> = {
  claim: (ref) => Promise.resolve(current(ref)),
  volume: () =>
    Promise.resolve({
      phase: 'Bound',
      capacity: '10Gi',
      accessModes: ['ReadWriteOnce'],
      volumeMode: 'Filesystem',
      className: 'fast-csi',
      claimName: 'app-data',
      claimNamespace: 'platform',
      claimUID: 'storage-claim-uid',
      reclaimPolicy: 'Retain',
      driver: 'example.csi.test',
      mountOptions: ['discard'],
      nodeAffinity: '',
      reason: '',
      message: '',
    }),
  storageClass: (ref) =>
    ref.clusterID === 'test-staging'
      ? Promise.reject(new Error('权限不足：不能读取 StorageClass'))
      : Promise.resolve(classValue()),
  previewClaim: async (request) => {
    const f = request.form;
    if (!f.name || !f.namespace || !f.capacity) throw new Error('PVC 名称、命名空间和容量不能为空');
    const ref = { ...request.target, name: f.name, namespace: f.namespace };
    const yaml = `apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: ${f.name}\n  namespace: ${f.namespace}\nspec:\n  accessModes: [${f.accessModes?.join(', ')}]\n  volumeMode: ${f.volumeMode}\n${f.classMode === 'default' ? '' : `  storageClassName: "${f.className}"\n`}  resources:\n    requests:\n      storage: ${f.capacity}\n`;
    const preview = await mutationFixtures.preview({
      target: ref,
      baseline: { uid: '', resourceVersion: '' },
      mode: 'create',
      revealSecret: false,
      yaml,
    });
    pending.set(preview.token, () =>
      claims.set(key(ref), {
        ...current(ref),
        phase: 'Pending',
        requested: f.capacity,
        capacity: '',
        volumeName: '',
        accessModes: f.accessModes,
        volumeMode: f.volumeMode,
        classMode: f.classMode,
        className: f.className,
      }),
    );
    return preview;
  },
  previewExpansion: async (request) => {
    if (request.target.clusterID === 'test-staging')
      throw new Error('权限不足：不能读取 StorageClass');
    const value = current(request.target);
    if (Number.parseFloat(request.capacity) <= Number.parseFloat(value.requested))
      throw new Error('扩容容量必须大于当前申请容量；不支持缩容');
    if (request.capacity === '99Gi') throw new Error('PVC 已变化或被重建，请重新读取基线后预检');
    const yaml = `apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: ${request.target.name}\n  namespace: ${request.target.namespace}\nspec:\n  storageClassName: fast-csi\n  resources:\n    requests:\n      storage: ${request.capacity}\n`;
    const preview = await mutationFixtures.preview({
      target: request.target,
      baseline: request.baseline,
      mode: 'patch',
      revealSecret: false,
      yaml,
    });
    pending.set(preview.token, () =>
      claims.set(key(request.target), {
        ...value,
        requested: request.capacity,
        conditions: [
          {
            type: 'FileSystemResizePending',
            status: 'True',
            reason: 'AwaitingFileSystem',
            message: '等待节点完成文件系统扩容',
          },
        ],
      }),
    );
    return preview;
  },
};

export function storageRows(query: Query): Page | null {
  const names =
    query.resource === 'persistentvolumeclaims'
      ? ['app-data', 'waiting-data']
      : query.resource === 'persistentvolumes'
        ? ['data-volume']
        : query.resource === 'storageclasses'
          ? query.continue
            ? ['fast-csi']
            : ['legacy-local']
          : null;
  if (!names) return null;
  return {
    items: names.map((name) => ({
      uid: `storage-${name}`,
      name,
      namespace: query.resource === 'persistentvolumeclaims' ? query.namespace || 'platform' : '',
      createdAt: '2026-10-24T10:00:00Z',
      status:
        name === 'waiting-data' ? 'Pending' : query.resource === 'storageclasses' ? '—' : 'Bound',
      detail: query.resource === 'storageclasses' ? 'example.csi.test' : '10Gi · fast-csi',
      labels: {},
    })),
    continue: query.resource === 'storageclasses' && !query.continue ? 'classes-page-2' : '',
    resourceVersion: '100',
  };
}

export function storageRelations(ref: Ref): RelationReport | null {
  if (!['persistentvolumeclaims', 'persistentvolumes', 'storageclasses'].includes(ref.resource))
    return null;
  const pod = {
    target: {
      ...ref,
      group: '',
      resource: 'pods',
      namespace: 'platform',
      name: 'storage-consumer',
    },
    uid: [ref.clusterID, '', 'v1', 'pods', 'platform', 'storage-consumer'].join('|'),
    kind: 'Pod',
    status: 'Running',
    detail: 'worker-01',
    revision: '',
    relation: 'PVC 卷引用',
  };
  const base = {
    items: [],
    events: [],
    note: '按需查询；绑定与挂载状态需分别核对。',
    error: '',
    truncated: false,
  };
  return {
    root: { ...pod, target: ref, uid: key(ref), kind: ref.resource, status: 'Bound' },
    checkedAt: new Date().toISOString(),
    sections: [
      {
        ...base,
        key: 'consumers',
        title: '引用此 PVC 的 Pod',
        items: ref.clusterID === 'test-staging' ? [] : [pod],
        error: ref.clusterID === 'test-staging' ? '权限不足：无法列出 Pod' : '',
      },
      {
        ...base,
        key: 'events',
        title: '当前资源事件',
        events: [
          {
            uid: 'storage-event',
            type: 'Normal',
            reason: 'WaitForFirstConsumer',
            message: '等待使用者参与调度',
            count: 1,
            time: '2026-10-27T12:00:00Z',
          },
        ],
      },
    ],
  };
}
