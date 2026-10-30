import { describe, expect, it } from 'vitest';
import type { DesktopAPI } from '../../api/types';
import { isStorage, listClasses } from './models';

describe('storage adapters', () => {
  it('requires the exact built-in group and version', () => {
    expect(isStorage({ group: '', version: 'v1', resource: 'persistentvolumeclaims' })).toBe(true);
    expect(
      isStorage({ group: 'example.test', version: 'v1', resource: 'persistentvolumeclaims' }),
    ).toBe(false);
    expect(isStorage({ group: 'storage.k8s.io', version: 'v2', resource: 'storageclasses' })).toBe(
      false,
    );
  });
  it('keeps cluster scope and exposes pagination loops instead of presenting a partial list', async () => {
    const calls: string[] = [];
    const api: Pick<DesktopAPI, 'list'> = {
      list: (query) => {
        expect(query.clusterID).toBe('cluster-a');
        expect(query.namespace).toBe('');
        expect(query.group).toBe('storage.k8s.io');
        calls.push(query.continue);
        return Promise.resolve({ items: [], continue: 'repeat', resourceVersion: '1' });
      },
    };
    await expect(listClasses(api, 'cluster-a', new AbortController().signal)).rejects.toThrow(
      '分页游标重复',
    );
    expect(calls).toEqual(['', 'repeat']);
  });
});
