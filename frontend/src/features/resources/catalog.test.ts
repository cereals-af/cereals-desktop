import { describe, expect, it } from 'vitest';
import { ageLabel, inCategory, typeKey } from './catalog';
import type { ResourceType } from '../../api/types';

const pod: ResourceType = {
  group: '',
  version: 'v1',
  resource: 'pods',
  kind: 'Pod',
  namespaced: true,
  verbs: ['list'],
};

describe('resource identity and categories', () => {
  it('distinguishes identical resource names in different API groups', () => {
    expect(typeKey(pod)).not.toBe(typeKey({ ...pod, group: 'custom.example.com' }));
  });
  it('keeps dynamically discovered resources accessible in the all category', () => {
    expect(inCategory({ ...pod, group: 'custom.example.com', resource: 'widgets' }, 'all')).toBe(
      true,
    );
    expect(inCategory(pod, 'workloads')).toBe(true);
    expect(inCategory(pod, 'storage')).toBe(false);
  });
  it('handles absent timestamps without claiming an age', () => {
    expect(ageLabel('')).toBe('—');
  });
});
