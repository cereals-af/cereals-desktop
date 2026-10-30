import { controllerFixtures, commitController, discardController } from './controllerFixtures';
import { commitNode, discardNode } from './nodeFixtures';
import { commitCustom, discardCustom } from './customFixtures';
import type { DesktopAPI, WorkloadRequest } from '../api/types';
import { current, key, saved } from './workloadState';
import { releaseFixtures, commitRelease, discardRelease } from './releaseFixtures';
import { mutationFixtures } from './mutationFixtures';
import { containerSnapshot } from './containerFixtures';
import { stringify } from 'yaml';

const pending = new Map<string, WorkloadRequest>();

export const workloadFixtures: Pick<
  DesktopAPI,
  | 'deployment'
  | 'previewWorkload'
  | 'commit'
  | 'discard'
  | keyof typeof releaseFixtures
  | keyof typeof controllerFixtures
> = {
  ...releaseFixtures,
  ...controllerFixtures,
  deployment: (target) =>
    target.name === 'restricted-scale'
      ? Promise.reject(new Error('没有读取 Deployment 的权限'))
      : Promise.resolve(current(target)),
  previewWorkload: async (request) => {
    if (
      !request.form.name ||
      !(request.form.containers ?? []).every((container) => container.image)
    )
      throw new Error('请填写名称和镜像');
    const target = { ...request.target, name: request.form.name };
    const text = stringify({
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      metadata: { name: target.name, namespace: target.namespace },
      spec: {
        replicas: request.form.replicas,
        template: {
          metadata: { annotations: { action: request.action } },
          spec: {
            containers: (request.form.containers ?? []).map((container) => ({
              name: container.name,
              image: container.image,
              command: container.command,
              args: container.args,
              ports: (container.ports ?? []).map((port) => ({
                name: port.name,
                protocol: port.protocol,
                containerPort: port.containerPort,
                hostPort: port.hostPort,
                hostIP: port.hostIP,
              })),
            })),
          },
        },
      },
    });
    const preview = await mutationFixtures.preview({
      target,
      mode: request.action === 'create' ? 'create' : 'patch',
      baseline: request.baseline,
      revealSecret: false,
      yaml: text,
    });
    pending.set(preview.token, structuredClone(request));
    return preview;
  },
  commit: async (token, signal) => {
    const release = commitRelease(token);
    if (release) return release;
    const request = pending.get(token);
    pending.delete(token);
    const result = await mutationFixtures.commit(token, signal);
    commitCustom(token);
    commitNode(token);
    commitController(token);
    if (request) {
      const snapshot = current(result.target);
      snapshot.form = request.form;
      snapshot.form.containers = containerSnapshot(snapshot.form.containers);
      if (request.action === 'pause' || request.action === 'resume')
        snapshot.status.paused = request.action === 'pause';
      snapshot.status.phase = '等待控制器处理';
      snapshot.status.observed = false;
      snapshot.baseline.resourceVersion = String(Number(snapshot.baseline.resourceVersion) + 1);
      saved.set(key(result.target), snapshot);
    }
    return result;
  },
  discard: (token) => {
    discardRelease(token);
    discardCustom(token);
    discardNode(token);
    discardController(token);
    pending.delete(token);
    return mutationFixtures.discard(token);
  },
};
