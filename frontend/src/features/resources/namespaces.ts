import type { DesktopAPI } from '../../api/types';

// Namespace lists use the same paginated API as other resources. Never stop at page one.
export async function listNamespaces(
  api: Pick<DesktopAPI, 'list'>,
  clusterID: string,
  signal: AbortSignal,
) {
  const names = new Set<string>();
  const cursors = new Set<string>();
  let cursor = '';
  do {
    signal.throwIfAborted();
    const page = await api.list(
      {
        clusterID,
        group: '',
        version: 'v1',
        resource: 'namespaces',
        namespace: '',
        labelSelector: '',
        continue: cursor,
      },
      signal,
    );
    for (const item of page.items ?? []) names.add(item.name);
    cursor = page.continue;
    if (cursor && cursors.has(cursor)) throw new Error('命名空间分页游标重复，请刷新重试');
    cursors.add(cursor);
  } while (cursor);
  return [...names].sort((a, b) => a.localeCompare(b));
}
