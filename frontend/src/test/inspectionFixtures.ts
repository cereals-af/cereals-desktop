import { fixturePodUID } from './sessionFixtures';
import type { DesktopAPI, Ref, RelatedNode, RelationSection } from '../api/types';
import { storageRelations } from './storageFixtures';

const kinds: Record<string, string> = {
  pods: 'Pod',
  statefulsets: 'StatefulSet',
  daemonsets: 'DaemonSet',
  deployments: 'Deployment',
  replicasets: 'ReplicaSet',
  services: 'Service',
  endpointslices: 'EndpointSlice',
  widgets: 'Widget',
  configmaps: 'ConfigMap',
};
function node(target: Ref, relation = '当前资源'): RelatedNode {
  return {
    target,
    kind: kinds[target.resource] ?? target.resource,
    uid: fixturePodUID(target),
    relation,
    status:
      target.resource === 'pods'
        ? target.name === 'api-gateway-pending'
          ? 'Pending'
          : 'Running'
        : target.resource === 'persistentvolumeclaims'
          ? 'Bound'
          : '1 / 2 就绪',
    detail: 'node-pool-1',
    revision: target.resource === 'replicasets' ? '3' : '',
    pod:
      target.resource === 'pods'
        ? { ready: '1 / 1', restarts: 2, containers: ['app', 'sidecar'] }
        : undefined,
  };
}
function section(key: string, title: string, items: RelatedNode[]): RelationSection {
  return {
    key,
    title,
    items,
    events: [],
    error: '',
    truncated: false,
    note: '通过资源 UID 核对关联；不同所有者不会混入。',
  };
}

export const inspectionFixtures: Pick<DesktopAPI, 'schema' | 'relations'> = {
  schema: (target) => {
    if (target.clusterID === 'test-staging')
      return Promise.reject(new Error('当前身份无法读取 OpenAPI 或 CRD Schema'));
    const container = {
      type: 'object',
      required: ['name', 'image'],
      additionalProperties: false,
      properties: {
        name: { type: 'string', description: '容器名称' },
        image: { type: 'string', description: '容器镜像地址' },
      },
    };
    const properties =
      target.resource === 'widgets'
        ? {
            sequence: { type: 'integer', format: 'int64', description: '64 位序号' },
            label: { type: 'string' },
          }
        : {
            containers: { type: 'array', items: container, description: '在 Pod 中运行的容器列表' },
            replicas: { type: 'integer', minimum: 0 },
          };
    return Promise.resolve({
      apiVersion: target.group ? `${target.group}/${target.version}` : target.version,
      kind: kinds[target.resource] ?? target.resource,
      source: 'OpenAPI v3',
      warnings: ['以服务端预检为准'],
      json: JSON.stringify({
        type: 'object',
        properties: {
          apiVersion: { type: 'string' },
          kind: { type: 'string' },
          metadata: { type: 'object' },
          spec: { type: 'object', properties, additionalProperties: false },
        },
      }),
    });
  },
  relations: (target) => {
    const storage = storageRelations(target);
    if (storage) return Promise.resolve(storage);
    const sections: RelationSection[] = [];
    if (target.resource === 'deployments') {
      sections.push(
        section('replicasets', 'ReplicaSet', [
          node({ ...target, resource: 'replicasets', name: 'api-gateway-rs-3' }, '控制器 UID'),
        ]),
      );
      sections.push(
        section('pods', 'Pod', [
          node(
            { ...target, group: '', resource: 'pods', name: 'api-gateway-pending' },
            '控制器 UID',
          ),
        ]),
      );
    }
    if (['statefulsets', 'daemonsets', 'replicasets'].includes(target.resource)) {
      const pods =
        target.name === 'empty-cache' || target.name === 'restricted-cache'
          ? []
          : [0, 1].map((i) =>
              node(
                { ...target, group: '', resource: 'pods', name: `${target.name}-${i}` },
                '控制器 UID',
              ),
            );
      const group = section('pods', 'Pod', pods);
      if (target.name === 'restricted-cache') group.error = '权限不足：无法列出 Pod';
      sections.push(group);
      if (target.resource === 'statefulsets')
        sections.push(
          section('claims', 'StatefulSet PVC', [
            node(
              { ...target, group: '', resource: 'persistentvolumeclaims', name: 'data-cache-0' },
              'PVC 模板命名规则',
            ),
          ]),
        );
    }
    if (target.resource === 'services') {
      const endpoint = node(
        {
          ...target,
          group: 'discovery.k8s.io',
          resource: 'endpointslices',
          name: 'gateway-endpoints',
        },
        'Service 名称标签',
      );
      endpoint.status = '1 / 2 就绪 · 1 未知';
      endpoint.detail = 'IPv4 · 10.244.0.10, 10.244.0.11';
      sections.push(section('endpointslices', 'EndpointSlice', [endpoint]));
      sections.push({
        ...section('pods', '选择器匹配的 Pod', []),
        error: '权限不足：无法列出 Pod',
      });
    }
    const events = section('events', '当前资源事件', []);
    if (target.resource === 'pods')
      events.events = [
        {
          uid: 'event-backoff',
          type: 'Warning',
          reason: 'BackOff',
          message: '容器镜像拉取失败，请检查镜像地址与镜像仓库权限。',
          count: 4,
          time: '2026-10-27T08:00:00Z',
        },
      ];
    sections.push(events);
    return Promise.resolve({ root: node(target), sections, checkedAt: new Date().toISOString() });
  },
};
