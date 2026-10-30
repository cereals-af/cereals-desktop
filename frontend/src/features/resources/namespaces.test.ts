import { expect, it, vi } from 'vitest';
import { listNamespaces } from './namespaces';
import type { DesktopAPI, Item } from '../../api/types';

const item = (name: string): Item => ({
  uid: name,
  name,
  namespace: '',
  createdAt: '',
  status: '',
  detail: '',
  labels: {},
});

it('loads every namespace page using the explicit cluster and cancellation signal', async () => {
  const list = vi
    .fn<DesktopAPI['list']>()
    .mockResolvedValueOnce({
      items: [item('z'), item('a')],
      continue: 'second',
      resourceVersion: '10',
    })
    .mockResolvedValueOnce({ items: [item('b'), item('a')], continue: '', resourceVersion: '10' });
  const controller = new AbortController();
  const api: Pick<DesktopAPI, 'list'> = { list };
  expect(await listNamespaces(api, 'cluster-a', controller.signal)).toEqual(['a', 'b', 'z']);
  expect(list).toHaveBeenLastCalledWith(
    expect.objectContaining({
      clusterID: 'cluster-a',
      resource: 'namespaces',
      namespace: '',
      continue: 'second',
    }),
    controller.signal,
  );
});

it('stops repeated cursors and propagates permission failures and cancellation', async () => {
  const list = vi
    .fn<DesktopAPI['list']>()
    .mockResolvedValue({ items: [], continue: 'loop', resourceVersion: '' });
  const controller = new AbortController();
  await expect(listNamespaces({ list }, 'one', controller.signal)).rejects.toThrow('游标重复');
  expect(list).toHaveBeenCalledTimes(2);
  list.mockRejectedValue(new Error('forbidden'));
  await expect(listNamespaces({ list }, 'one', controller.signal)).rejects.toThrow('forbidden');
  controller.abort();
  list.mockClear();
  await expect(listNamespaces({ list }, 'one', controller.signal)).rejects.toThrow();
  expect(list).not.toHaveBeenCalled();
});
