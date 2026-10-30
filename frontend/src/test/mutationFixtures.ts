import type { DesktopAPI, Item, ManifestRequest, Preview, Query, Ref } from '../api/types';

const objects = new Map<string, string>();
const deleted = new Set<string>();
const pending = new Map<string, Preview>();
const listeners = new Map<string, number>();
let revision = 100;
let sequence = 0;
const key = (ref: Ref) =>
  [ref.clusterID, ref.group, ref.version, ref.resource, ref.namespace, ref.name].join('|');
const kinds: Record<string, string> = {
  pods: 'Pod',
  deployments: 'Deployment',
  services: 'Service',
  widgets: 'Widget',
  configmaps: 'ConfigMap',
};

export function fixtureYAML(ref: Ref) {
  const stored = objects.get(key(ref));
  if (stored !== undefined) return stored;
  if (ref.resource === 'nodes')
    return `apiVersion: v1\nkind: Node\nmetadata:\n  name: ${ref.name}\nspec:\n  unschedulable: false\n`;
  if (ref.resource === 'persistentvolumeclaims')
    return `apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: ${ref.name}\n  namespace: ${ref.namespace}\nspec:\n  storageClassName: fast-csi\n  resources:\n    requests:\n      storage: 10Gi\n`;
  return `apiVersion: ${ref.group ? `${ref.group}/` : ''}${ref.version}\nkind: ${kinds[ref.resource] ?? 'Pod'}\nmetadata:\n  name: ${ref.name}\n  namespace: ${ref.namespace}\nspec:\n  containers:\n    - name: app\n      image: nginx:stable\n`;
}

export function fixtureRows(query: Query, items: Item[]): Item[] {
  const scope = key({ ...query, name: '' });
  const rows = items.filter((item) => !deleted.has(key({ ...query, name: item.name })));
  for (const stored of objects.keys()) {
    if (!stored.startsWith(scope)) continue;
    const name = stored.slice(scope.length);
    if (!rows.some((row) => row.name === name))
      rows.push({
        uid: stored,
        name,
        namespace: query.namespace,
        createdAt: new Date().toISOString(),
        status: 'Active',
        detail: '新建资源',
        labels: {},
      });
  }
  return rows;
}

function makePreview(request: ManifestRequest): Preview {
  if (request.yaml.includes('image: invalid-preview'))
    throw new Error('资源校验失败：spec.containers[0].image');
  const name =
    request.target.name || request.yaml.match(/^ {2}name: (.+)$/m)?.[1] || 'new-resource';
  const target = { ...request.target, name };
  const preview: Preview = {
    token: `preview-${++sequence}`,
    target,
    mode: request.mode,
    before: request.mode === 'create' ? '' : fixtureYAML(target),
    desired: request.yaml,
    after: request.yaml,
    expiresAt: new Date(Date.now() + 300000).toISOString(),
  };
  pending.set(preview.token, preview);
  return preview;
}

function wait(signal?: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const finish = () => {
      signal?.removeEventListener('abort', abort);
      resolve();
    };
    const timer = setTimeout(finish, 500);
    const abort = () => {
      clearTimeout(timer);
      signal?.removeEventListener('abort', abort);
      reject(new Error('请求已取消'));
    };
    signal?.addEventListener('abort', abort, { once: true });
    if (signal?.aborted) abort();
  });
}

export const mutationFixtures: Pick<
  DesktopAPI,
  | 'snapshot'
  | 'preview'
  | 'previewDelete'
  | 'commit'
  | 'discard'
  | 'subscribe'
  | 'poll'
  | 'unsubscribe'
> = {
  snapshot: (ref, reveal) =>
    Promise.resolve({
      yaml: fixtureYAML(ref),
      editable: ref.resource !== 'secrets' || reveal,
      baseline: { uid: key(ref), resourceVersion: String(revision) },
    }),
  preview: (request) => Promise.resolve(makePreview(request)),
  previewDelete: (request) => {
    const preview: Preview = {
      token: `delete-${++sequence}`,
      target: request.target,
      mode: 'delete',
      before: fixtureYAML(request.target),
      desired: '',
      after: '',
      expiresAt: new Date(Date.now() + 300000).toISOString(),
    };
    pending.set(preview.token, preview);
    return Promise.resolve(preview);
  },
  commit: (token) => {
    const preview = pending.get(token);
    pending.delete(token);
    if (!preview) return Promise.reject(new Error('预检已失效'));
    if (preview.desired.includes('image: conflict'))
      return Promise.reject(new Error('资源版本或 SSA 字段归属冲突'));
    if (preview.mode === 'delete') {
      deleted.add(key(preview.target));
      objects.delete(key(preview.target));
    } else {
      deleted.delete(key(preview.target));
      objects.set(key(preview.target), preview.desired);
    }
    revision++;
    return Promise.resolve({
      target: preview.target,
      message: 'API Server 已接受请求；请通过资源状态确认实际结果。',
    });
  },
  discard: (token) => {
    pending.delete(token);
    return Promise.resolve();
  },
  subscribe: () => {
    const id = `watch-${++sequence}`;
    listeners.set(id, revision);
    return Promise.resolve(id);
  },
  poll: async (id, _revision, signal) => {
    await wait(signal);
    if (!listeners.has(id)) throw new Error('实时订阅已过期');
    return {
      revision,
      changes: revision - (listeners.get(id) ?? revision),
      state: 'live',
      message: '',
    };
  },
  unsubscribe: (id) => {
    listeners.delete(id);
    return Promise.resolve();
  },
};
