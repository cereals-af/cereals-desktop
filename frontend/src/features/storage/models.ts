import type { DesktopAPI, Ref, StorageClaim, StorageVolume, StorageClass } from '../../api/types';

export type StorageDocument =
  | { kind: 'claim'; value: StorageClaim }
  | { kind: 'volume'; value: StorageVolume }
  | { kind: 'class'; value: StorageClass };

export function isStorage(type: { group: string; version: string; resource: string }) {
  return (
    type.version === 'v1' &&
    ((type.group === '' &&
      ['persistentvolumeclaims', 'persistentvolumes'].includes(type.resource)) ||
      (type.group === 'storage.k8s.io' && type.resource === 'storageclasses'))
  );
}

export async function loadStorage(
  api: DesktopAPI,
  target: Ref,
  signal: AbortSignal,
): Promise<StorageDocument> {
  if (target.resource === 'persistentvolumeclaims')
    return { kind: 'claim', value: await api.claim(target, signal) };
  if (target.resource === 'persistentvolumes')
    return { kind: 'volume', value: await api.volume(target, signal) };
  return { kind: 'class', value: await api.storageClass(target, signal) };
}

export const classRef = (clusterID: string, name: string): Ref => ({
  clusterID,
  group: 'storage.k8s.io',
  version: 'v1',
  resource: 'storageclasses',
  namespace: '',
  name,
});

export async function listClasses(
  api: Pick<DesktopAPI, 'list'>,
  clusterID: string,
  signal: AbortSignal,
) {
  const names = new Set<string>();
  const cursors = new Set<string>();
  let cursor = '';
  for (let pageNumber = 0; pageNumber < 50; pageNumber++) {
    signal.throwIfAborted();
    const page = await api.list(
      { ...classRef(clusterID, ''), labelSelector: '', continue: cursor },
      signal,
    );
    for (const item of page.items ?? []) names.add(item.name);
    cursor = page.continue;
    if (!cursor) return [...names].sort();
    if (cursors.has(cursor)) throw new Error('StorageClass 分页游标重复，请刷新重试');
    cursors.add(cursor);
  }
  throw new Error('StorageClass 列表超过扫描上限，请按名称指定');
}
