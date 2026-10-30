import { visualFixtures, visualRows } from './visualFixtures';
import { metricsFixtures } from './metricsFixtures';
import { nodeFixtures, nodeRows } from './nodeFixtures';
import { storageFixtures, storageRows } from './storageFixtures';
import type { DesktopAPI, Profile, ResourceType } from '../api/types';
import { fixtureRows, fixtureYAML, mutationFixtures } from './mutationFixtures';
import { workloadFixtures } from './workloadFixtures';
import { customFixtures } from './customFixtures';
import { coreFixtures } from './coreFixtures';
import { transferFixtures } from './transferFixtures';
import { sessionFixtures } from './sessionFixtures';
import { inspectionFixtures } from './inspectionFixtures';

// Development-only deterministic fixtures. No network requests or real credentials.
const profiles: Profile[] = [
  {
    id: 'test-production',
    context: 'production-shanghai',
    cluster: 'production',
    server: 'https://192.0.2.10:6443',
    namespace: 'platform',
    source: '/fixtures/config',
    execCommand: '',
    insecureTLS: false,
    connected: false,
    version: '',
  },
  {
    id: 'test-staging',
    context: 'staging-hangzhou',
    cluster: 'staging',
    server: 'https://192.0.2.20:6443',
    namespace: 'default',
    source: '/fixtures/config',
    execCommand: '',
    insecureTLS: false,
    connected: false,
    version: '',
  },
  {
    id: 'test-plugin',
    context: 'cloud-auth',
    cluster: 'cloud',
    server: 'https://192.0.2.30:6443',
    namespace: 'default',
    source: '/fixtures/config',
    execCommand: 'test-auth-plugin',
    insecureTLS: false,
    connected: false,
    version: '',
  },
];
const resources: ResourceType[] = [
  ...[
    { group: 'batch', resource: 'jobs', kind: 'Job' },
    { group: 'batch', resource: 'cronjobs', kind: 'CronJob' },
    { group: 'networking.k8s.io', resource: 'ingresses', kind: 'Ingress' },
    { group: 'networking.k8s.io', resource: 'networkpolicies', kind: 'NetworkPolicy' },
    { group: '', resource: 'secrets', kind: 'Secret' },
  ].map((r) => ({
    ...r,
    version: 'v1',
    namespaced: true,
    verbs: ['get', 'list', 'create', 'patch', 'delete'],
  })),
  ...[
    { resource: 'statefulsets', kind: 'StatefulSet' },
    { resource: 'daemonsets', kind: 'DaemonSet' },
    { resource: 'replicasets', kind: 'ReplicaSet' },
  ].map((r) => ({
    ...r,
    group: 'apps',
    version: 'v1',
    namespaced: true,
    verbs: ['get', 'list', 'create', 'patch', 'delete', 'watch'],
  })),
  {
    group: '',
    version: 'v1',
    resource: 'persistentvolumeclaims',
    kind: 'PersistentVolumeClaim',
    namespaced: true,
    verbs: ['get', 'list', 'watch', 'create', 'patch', 'delete'],
  },
  {
    group: '',
    version: 'v1',
    resource: 'persistentvolumes',
    kind: 'PersistentVolume',
    namespaced: false,
    verbs: ['get', 'list', 'create', 'patch', 'delete'],
  },
  {
    group: 'storage.k8s.io',
    version: 'v1',
    resource: 'storageclasses',
    kind: 'StorageClass',
    namespaced: false,
    verbs: ['get', 'list', 'create', 'patch', 'delete'],
  },
  {
    group: '',
    version: 'v1',
    resource: 'nodes',
    kind: 'Node',
    namespaced: false,
    verbs: ['get', 'list', 'patch'],
  },
  {
    group: 'apiextensions.k8s.io',
    version: 'v1',
    resource: 'customresourcedefinitions',
    kind: 'CustomResourceDefinition',
    namespaced: false,
    verbs: ['get', 'list', 'create', 'patch', 'delete'],
  },
  {
    group: '',
    version: 'v1',
    resource: 'namespaces',
    kind: 'Namespace',
    namespaced: false,
    verbs: ['get', 'list'],
  },
  {
    group: '',
    version: 'v1',
    resource: 'pods',
    kind: 'Pod',
    namespaced: true,
    verbs: ['get', 'list', 'watch', 'create', 'patch', 'delete'],
  },
  {
    group: 'apps',
    version: 'v1',
    resource: 'deployments',
    kind: 'Deployment',
    namespaced: true,
    verbs: ['get', 'list', 'watch', 'create', 'patch', 'delete'],
  },
  {
    group: '',
    version: 'v1',
    resource: 'services',
    kind: 'Service',
    namespaced: true,
    verbs: ['get', 'list', 'watch', 'create', 'patch', 'delete'],
  },
  {
    group: '',
    version: 'v1',
    resource: 'configmaps',
    kind: 'ConfigMap',
    namespaced: true,
    verbs: ['get', 'list', 'watch', 'create', 'patch', 'delete'],
  },
  {
    group: 'demo.example.com',
    version: 'v1',
    resource: 'widgets',
    kind: 'Widget',
    namespaced: true,
    verbs: ['get', 'list', 'create', 'patch'],
  },
];
let importedIDs = (sessionStorage.getItem('fixture-clusters') ?? '')
  .split(',')
  .filter((id) => profiles.some((p) => p.id === id));
let defaultID = sessionStorage.getItem('fixture-default') ?? '';
let approvedID = sessionStorage.getItem('fixture-approved') ?? '';
const state = () => {
  const visible = profiles.filter((p) => importedIDs.includes(p.id));
  const profile = visible.length === 1 ? visible[0] : visible.find((p) => p.id === defaultID);
  return {
    profiles: structuredClone(visible),
    warnings: [],
    defaultClusterID: profile?.id ?? '',
    defaultNeedsConfirmation: Boolean(
      profile && (profile.execCommand || profile.insecureTLS) && approvedID !== profile.id,
    ),
  };
};
function importFixtures() {
  importedIDs = profiles.map((p) => p.id);
  sessionStorage.setItem('fixture-clusters', importedIDs.join(','));
  return state();
}

function requireConnection(id: string) {
  const profile = profiles.find((item) => item.id === id);
  if (!profile?.connected) throw new Error('测试集群未连接');
  return profile;
}

export const fixtureAPI: DesktopAPI = {
  ...metricsFixtures,
  setDefaultCluster: (id) => {
    const profile = profiles.find((p) => p.id === id && importedIDs.includes(p.id));
    if (!profile) return Promise.reject(new Error('默认集群不存在'));
    defaultID = id;
    if (profile.connected) approvedID = id;
    sessionStorage.setItem('fixture-default', id);
    sessionStorage.setItem('fixture-approved', approvedID);
    return Promise.resolve(state());
  },
  clusterOverview: (id, signal) => {
    requireConnection(id);
    if (signal?.aborted) return Promise.reject(new Error('请求已取消'));
    const staging = id === 'test-staging';
    return Promise.resolve({
      clusterID: id,
      updatedAt: new Date().toISOString(),
      nodes: staging
        ? null
        : {
            count: 3,
            ready: 2,
            unschedulable: 1,
            capacity: {
              cpu: '24',
              memory: '100663296Ki',
              'ephemeral-storage': '600Gi',
              pods: '330',
            },
            allocatable: { cpu: '22.5', memory: '90Gi', 'ephemeral-storage': '550Gi', pods: '330' },
          },
      nodeError: staging ? '权限不足：当前身份无权列出 Node' : '',
      warnings: [],
      resources: resources.map((type, index) => {
        const failed = staging && ['namespaces', 'nodes', 'secrets'].includes(type.resource);
        return {
          type,
          count: failed ? null : type.resource === 'nodes' ? 3 : index,
          error: failed ? '权限不足：当前身份无权在集群范围统计此资源' : '',
        };
      }),
    });
  },
  ...mutationFixtures,
  ...visualFixtures,
  ...customFixtures,
  ...nodeFixtures,
  ...storageFixtures,
  ...workloadFixtures,
  ...coreFixtures,
  ...inspectionFixtures,
  ...sessionFixtures,
  ...transferFixtures,
  state: () => Promise.resolve(state()),
  importFile: () => {
    return Promise.resolve(importFixtures());
  },
  importDefault: () => {
    return Promise.resolve(importFixtures());
  },
  connect: (request) => {
    const profile = profiles.find((item) => item.id === request.clusterID);
    if (!profile) return Promise.reject(new Error('集群不存在'));
    if (profile.execCommand && !request.trustExec)
      return Promise.reject(new Error('请确认信任认证插件'));
    profile.connected = true;
    profile.version = 'v1.36.2';
    if (state().defaultClusterID === profile.id) {
      approvedID = profile.id;
      sessionStorage.setItem('fixture-approved', profile.id);
    }
    return Promise.resolve({ clusterID: profile.id, version: profile.version, latencyMS: 12 });
  },
  disconnect: (id) => {
    requireConnection(id).connected = false;
    return Promise.resolve();
  },
  discover: (id) => {
    requireConnection(id);
    return Promise.resolve({ resources, warnings: [] });
  },
  list: (query) => {
    requireConnection(query.clusterID);
    const visual = visualRows(query);
    if (visual)
      return Promise.resolve({
        items: fixtureRows(query, visual),
        continue: '',
        resourceVersion: '100',
      });
    const storage = storageRows(query);
    if (storage) {
      if (query.resource === 'storageclasses' && query.clusterID === 'test-staging')
        return Promise.reject(new Error('权限不足：无法列出 StorageClass'));
      return Promise.resolve(storage);
    }
    if (query.resource === 'nodes')
      return Promise.resolve({
        items: nodeRows(query.clusterID),
        continue: '',
        resourceVersion: '10',
      });
    if (query.resource === 'namespaces') {
      if (query.clusterID === 'test-staging')
        return Promise.reject(new Error('权限不足：当前身份不能列出 Namespace'));
      const names = query.continue ? ['kube-system', 'team-backend'] : ['platform', 'default'];
      return Promise.resolve({
        items: names.map((name) => ({
          uid: name,
          name,
          namespace: '',
          createdAt: '',
          status: 'Active',
          detail: '',
          labels: {},
        })),
        continue: query.continue ? '' : 'namespaces-page-2',
        resourceVersion: '100',
      });
    }
    if (query.namespace === 'forbidden')
      return Promise.reject(new Error('权限不足：当前身份无权执行此操作'));
    const names =
      query.resource === 'statefulsets'
        ? ['cache', 'empty-cache', 'restricted-cache']
        : query.resource === 'daemonsets'
          ? ['node-agent', 'ondelete-agent']
          : query.resource === 'replicasets'
            ? ['managed-rs', 'standalone-rs']
            : query.resource === 'customresourcedefinitions'
              ? ['widgets.demo.example.com']
              : query.resource === 'services'
                ? ['gateway-service']
                : query.resource === 'configmaps'
                  ? ['app-config', 'immutable-config']
                  : query.resource === 'deployments'
                    ? ['api-gateway', 'console']
                    : query.resource === 'widgets'
                      ? ['sample-widget']
                      : [
                          'api-gateway-6f8d7c9b4-x2k5m',
                          'console-7b9f8d6c5-p8r2n',
                          'worker-5d7c8f9b6-k4m7q',
                          'scheduler-8c6f7d9b5-n3p6r',
                          'cache-0',
                          'metrics-collector-9b7d6c8f5-h2j4k',
                        ];
    const prefix = query.clusterID === 'test-staging' ? 'staging-' : '';
    return Promise.resolve({
      items: fixtureRows(
        query,
        names
          .filter((name) => !query.labelSelector || name.startsWith('api-'))
          .map((name, index) => ({
            uid: `${query.clusterID}-${index}`,
            name: prefix + name,
            namespace: query.namespace || 'platform',
            createdAt: '2026-10-24T10:00:00Z',
            status:
              query.resource === 'pods'
                ? (['Ready', 'Completed', 'Processing', 'Processing', 'Error', 'Ready'][index] ??
                  'Ready')
                : query.resource === 'deployments' && index === 1
                  ? 'Stopped'
                  : index === 3
                    ? 'Processing'
                    : 'Ready',
            detail: `node-pool-${(index % 3) + 1}`,
            labels: { app: name.split('-')[0] ?? name },
          })),
      ),
      continue: query.continue ? '' : 'next-page',
      resourceVersion: '100',
    });
  },
  yaml: (ref) => {
    requireConnection(ref.clusterID);
    return Promise.resolve(fixtureYAML(ref));
  },
};
