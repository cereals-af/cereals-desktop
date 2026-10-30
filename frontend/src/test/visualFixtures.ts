import type { DesktopAPI, Query, Ref, VisualSnapshot } from '../api/types';
import { property, setProperty, valueOf } from '../features/custom-resources/values';
import { newSnapshot, tree, visualKind } from '../features/visual-forms/models';
import { mutationFixtures } from './mutationFixtures';

const snapshots = new Map<string, VisualSnapshot>();
const pending = new Map<string, () => void>();
const key = (target: Ref) => JSON.stringify(target);
export function visualRows(query: Query) {
  if (!visualKind(query)) return undefined;
  return [
    {
      uid: `visual-${query.resource}`,
      name: `demo-${query.resource}`,
      namespace: query.namespace || 'platform',
      createdAt: '2026-10-28T00:00:00Z',
      status: 'Active',
      detail: '可视化测试资源',
      labels: {},
    },
  ];
}
function snapshot(target: Ref): VisualSnapshot {
  const kind = visualKind(target);
  if (!kind) throw new Error('不支持的资源');
  const result = structuredClone(snapshots.get(key(target)) ?? newSnapshot(kind, target));
  result.value = setProperty(result.value, 'name', valueOf('string', target.name));
  result.baseline = { uid: `visual-${target.name}`, resourceVersion: '100' };
  result.status =
    kind === 'Job' || kind === 'CronJob' ? tree({ active: 0, succeeded: 1 }) : tree({});
  if (kind === 'Secret' && !snapshots.has(key(target)))
    result.entries = [
      { key: 'application.yaml', value: '服务:\n  密码: 测试密钥\n', encoding: 'text' },
      { key: 'binary', value: 'AP8=', encoding: 'base64' },
    ];
  return result;
}
export const visualFixtures: Pick<DesktopAPI, 'visualSnapshot' | 'previewVisual' | 'previewRun'> = {
  visualSnapshot: (target, reveal) => {
    const result = snapshot(target);
    result.revealed = reveal;
    if (!reveal && target.resource === 'secrets')
      result.entries = result.entries?.map((e) => ({ ...e, value: '' })) ?? [];
    return Promise.resolve(result);
  },
  previewVisual: async (request) => {
    const name = property(request.value, 'name')?.text;
    if (!name) throw new Error('请填写资源名称');
    const target = { ...request.target, name };
    const p = await mutationFixtures.preview({
      target,
      mode: request.mode,
      baseline: request.baseline,
      revealSecret: request.revealed,
      yaml: JSON.stringify(
        {
          apiVersion: target.group ? `${target.group}/v1` : 'v1',
          kind: visualKind(target),
          metadata: { name, namespace: target.namespace },
          form: request.value,
        },
        null,
        2,
      ),
    });
    pending.set(p.token, () =>
      snapshots.set(key(target), {
        value: request.value,
        entries: request.entries ?? [],
        revealed: request.revealed,
        baseline: { uid: `visual-${name}`, resourceVersion: '101' },
        status: tree({ active: 0 }),
      }),
    );
    return p;
  },
  previewRun: async (request) => {
    const target = { ...request.target, resource: 'jobs', name: request.name };
    return mutationFixtures.preview({
      target,
      mode: 'create',
      baseline: { uid: '', resourceVersion: '' },
      revealSecret: false,
      yaml: JSON.stringify({
        apiVersion: 'batch/v1',
        kind: 'Job',
        metadata: { name: request.name, namespace: target.namespace },
      }),
    });
  },
};
export function commitVisual(token: string) {
  pending.get(token)?.();
  pending.delete(token);
}
export function discardVisual(token: string) {
  pending.delete(token);
}
