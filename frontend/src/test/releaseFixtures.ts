import type { DesktopAPI, Preview, Ref, MutationResult } from '../api/types';
import { current, key, saved } from './workloadState';

type Change = { target: Ref; replicas: number } | { target: Ref; image: string };
const pending = new Map<string, Change>();
let sequence = 0;
function preview(target: Ref, before: string, after: string, change: Change): Preview {
  const token = `release-${++sequence}`;
  pending.set(token, change);
  return {
    token,
    target,
    mode: 'patch',
    before,
    desired: after,
    after,
    expiresAt: new Date(Date.now() + 300000).toISOString(),
  };
}
export function commitRelease(token: string): MutationResult | null {
  const change = pending.get(token);
  if (!change) return null;
  pending.delete(token);
  const snapshot = current(change.target);
  if ('replicas' in change) snapshot.form.replicas = change.replicas;
  else for (const container of snapshot.form.containers ?? []) container.image = change.image;
  snapshot.status.phase = '等待控制器处理';
  snapshot.status.observed = false;
  snapshot.baseline.resourceVersion = String(Number(snapshot.baseline.resourceVersion) + 1);
  saved.set(key(change.target), snapshot);
  return { target: change.target, message: 'API Server 已接受请求；请通过资源状态确认实际结果。' };
}
export function discardRelease(token: string) {
  pending.delete(token);
}
export const releaseFixtures: Pick<
  DesktopAPI,
  'scale' | 'previewScale' | 'deploymentHistory' | 'previewRollback' | 'workloadPermissions'
> = {
  scale: (target) => {
    const snapshot = current(target);
    return Promise.resolve({
      baseline: snapshot.baseline,
      replicas: snapshot.form.replicas,
      actual: snapshot.status.replicas,
      selector: 'app=api-gateway',
    });
  },
  previewScale: (request) => {
    const snapshot = current(request.target);
    const yaml = (replicas: number) =>
      `apiVersion: autoscaling/v1\nkind: Scale\nmetadata:\n  name: ${request.target.name}\n  namespace: ${request.target.namespace}\nspec:\n  replicas: ${replicas}\n`;
    return Promise.resolve(
      preview(request.target, yaml(snapshot.form.replicas), yaml(request.replicas), {
        target: request.target,
        replicas: request.replicas,
      }),
    );
  },
  deploymentHistory: (target) => {
    const snapshot = current(target);
    return Promise.resolve({
      baseline: snapshot.baseline,
      currentRevision: '5',
      paused: snapshot.status.paused,
      warnings: [],
      truncated: false,
      revisions: [
        {
          name: `${target.name}-current`,
          baseline: { uid: 'rs-current', resourceVersion: '103' },
          revision: '5',
          createdAt: '2026-10-26T10:00:00Z',
          images: ['app: nginx:1.29'],
          replicas: 3,
          ready: 3,
          current: snapshot.form.containers?.[0]?.image !== 'nginx:1.28',
          unavailable: '',
        },
        {
          name: `${target.name}-previous`,
          baseline: { uid: 'rs-previous', resourceVersion: '101' },
          revision: '3',
          createdAt: '2026-10-25T10:00:00Z',
          images: ['app: nginx:1.28'],
          replicas: 0,
          ready: 0,
          current: snapshot.form.containers?.[0]?.image === 'nginx:1.28',
          unavailable: '',
        },
      ],
    });
  },
  previewRollback: (request) => {
    const snapshot = current(request.target);
    if (snapshot.status.paused)
      return Promise.reject(new Error('Deployment 已暂停，请先恢复发布后再回滚'));
    return Promise.resolve(
      preview(
        request.target,
        `image: ${snapshot.form.containers?.[0]?.image}\n`,
        'image: nginx:1.28\n',
        { target: request.target, image: 'nginx:1.28' },
      ),
    );
  },
  workloadPermissions: (target) =>
    Promise.resolve([
      {
        verb: 'patch',
        subresource: '',
        state: target.name === 'restricted-scale' ? 'denied' : 'allowed',
        message: '',
      },
      ...['get', 'update', 'patch'].map((verb) => ({
        verb,
        subresource: 'scale',
        state: 'allowed',
        message: '',
      })),
    ]),
};
