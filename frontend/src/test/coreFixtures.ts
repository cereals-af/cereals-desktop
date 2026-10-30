import { commitVisual, discardVisual } from './visualFixtures';
import type { ConfigMapSnapshot, DesktopAPI, Ref, ServiceSnapshot } from '../api/types';
import { newConfigMap, newPort, newService } from '../features/service-config/models';
import { mutationFixtures } from './mutationFixtures';
import { workloadFixtures } from './workloadFixtures';
import { commitStorage, discardStorage } from './storageFixtures';

const services = new Map<string, ServiceSnapshot>();
const configs = new Map<string, ConfigMapSnapshot>();
const pending = new Map<string, () => void>();
const key = (target: Ref) => [target.clusterID, target.namespace, target.name].join('/');

function serviceSnapshot(target: Ref): ServiceSnapshot {
  return structuredClone(
    services.get(key(target)) ?? {
      baseline: { uid: key(target), resourceVersion: '100' },
      clusterIPs: ['10.96.0.12'],
      ipFamilies: ['IPv4'],
      loadBalancerAddresses: [],
      form: {
        ...newService(target.namespace),
        name: target.name,
        type: 'NodePort',
        ports: [{ ...newPort(), originalKey: 'name:http', nodePort: 30080 }],
        selector: [{ key: 'app', value: 'gateway' }],
      },
    },
  );
}

function configSnapshot(target: Ref): ConfigMapSnapshot {
  return structuredClone(
    configs.get(key(target)) ?? {
      baseline: { uid: key(target), resourceVersion: '100' },
      form: {
        ...newConfigMap(target.namespace),
        name: target.name,
        immutable: target.name === 'immutable-config',
        data: [{ key: 'application.yaml', value: 'server:\n  port: 8080\n' }],
        binaryData: [{ key: 'payload.bin', value: 'AAEC/w==' }],
      },
    },
  );
}

export const coreFixtures: Pick<
  DesktopAPI,
  'service' | 'previewService' | 'configMap' | 'previewConfigMap' | 'commit' | 'discard'
> = {
  service: (target) => Promise.resolve(serviceSnapshot(target)),
  configMap: (target) => Promise.resolve(configSnapshot(target)),
  previewService: async (request) => {
    if (!request.form.name) throw new Error('请填写 Service 名称');
    const target = { ...request.target, name: request.form.name };
    const form = structuredClone(request.form);
    if (form.type === 'ClusterIP')
      form.ports = form.ports?.map((port) => ({ ...port, nodePort: 0 })) ?? [];
    const desired = JSON.stringify(
      {
        apiVersion: 'v1',
        kind: 'Service',
        metadata: { name: target.name, namespace: target.namespace },
        spec: form,
      },
      null,
      2,
    );
    const preview = await mutationFixtures.preview({
      target,
      mode: request.mode,
      baseline: request.baseline,
      revealSecret: false,
      yaml: desired,
    });
    pending.set(preview.token, () =>
      services.set(key(target), {
        ...serviceSnapshot(target),
        form,
        baseline: { uid: key(target), resourceVersion: '101' },
      }),
    );
    return preview;
  },
  previewConfigMap: async (request) => {
    if (!request.form.name) throw new Error('请填写 ConfigMap 名称');
    const seen = new Set<string>();
    for (const entry of [...(request.form.data ?? []), ...(request.form.binaryData ?? [])]) {
      if (!entry.key || seen.has(entry.key)) throw new Error('配置键名不能为空或重复');
      seen.add(entry.key);
    }
    for (const entry of request.form.binaryData ?? []) {
      try {
        atob(entry.value);
      } catch {
        throw new Error('binaryData 必须填写有效的标准 Base64');
      }
    }
    const form = structuredClone(request.form);
    const target = { ...request.target, name: form.name };
    const desired = JSON.stringify(
      {
        apiVersion: 'v1',
        kind: 'ConfigMap',
        metadata: { name: target.name, namespace: target.namespace },
        data: Object.fromEntries((form.data ?? []).map((entry) => [entry.key, entry.value])),
        binaryData: Object.fromEntries(
          (form.binaryData ?? []).map((entry) => [entry.key, entry.value]),
        ),
        immutable: form.immutable,
      },
      null,
      2,
    );
    const preview = await mutationFixtures.preview({
      target,
      mode: request.mode,
      baseline: request.baseline,
      revealSecret: false,
      yaml: desired,
    });
    pending.set(preview.token, () =>
      configs.set(key(target), { form, baseline: { uid: key(target), resourceVersion: '101' } }),
    );
    return preview;
  },
  commit: async (token, signal) => {
    const action = pending.get(token);
    pending.delete(token);
    const result = await workloadFixtures.commit(token, signal);
    commitStorage(token);
    commitVisual(token);
    action?.();
    return result;
  },
  discard: (token) => {
    discardStorage(token);
    discardVisual(token);
    pending.delete(token);
    return workloadFixtures.discard(token);
  },
};
