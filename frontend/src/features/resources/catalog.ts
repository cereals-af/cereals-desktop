import type { ResourceType } from '../../api/types';

export type Category =
  'all' | 'workloads' | 'network' | 'storage' | 'configuration' | 'access' | 'nodes' | 'custom';
export type View = 'connections' | Category;

export const categoryLabels: Record<View, string> = {
  connections: '集群连接',
  all: '资源浏览器',
  workloads: '工作负载',
  network: '服务与网络',
  storage: '存储管理',
  configuration: '配置管理',
  access: '访问控制',
  nodes: '节点与空间',
  custom: '自定义资源',
};

const resourceNames: Record<string, string> = {
  pods: '容器组',
  deployments: '部署',
  statefulsets: '有状态副本集',
  daemonsets: '守护进程集',
  replicasets: '副本集',
  jobs: '任务',
  cronjobs: '定时任务',
  services: '服务',
  ingresses: '应用路由',
  endpointslices: '端点切片',
  networkpolicies: '网络策略',
  configmaps: '配置字典',
  secrets: '保密字典',
  persistentvolumeclaims: '存储卷声明',
  persistentvolumes: '持久卷',
  storageclasses: '存储类型',
  nodes: '节点',
  namespaces: '命名空间',
  customresourcedefinitions: '自定义资源定义',
};

const sets: Record<Exclude<Category, 'all' | 'custom'>, Set<string>> = {
  workloads: new Set([
    'pods',
    'deployments',
    'statefulsets',
    'daemonsets',
    'replicasets',
    'jobs',
    'cronjobs',
    'horizontalpodautoscalers',
    'poddisruptionbudgets',
  ]),
  network: new Set([
    'services',
    'ingresses',
    'ingressclasses',
    'endpoints',
    'endpointslices',
    'networkpolicies',
  ]),
  storage: new Set([
    'persistentvolumeclaims',
    'persistentvolumes',
    'storageclasses',
    'csidrivers',
    'csinodes',
    'volumeattachments',
  ]),
  configuration: new Set(['configmaps', 'secrets', 'resourcequotas', 'limitranges']),
  access: new Set([
    'serviceaccounts',
    'roles',
    'rolebindings',
    'clusterroles',
    'clusterrolebindings',
  ]),
  nodes: new Set(['nodes', 'namespaces', 'events', 'priorityclasses', 'runtimeclasses']),
};

// CRD names are discovered from the cluster; this category is explicitly labelled
// as extensions and includes aggregated APIs, not just guessed custom groups.
export function inCategory(type: ResourceType, category: Category): boolean {
  if (category === 'all') return true;
  if (category === 'custom') {
    return (
      type.resource === 'customresourcedefinitions' ||
      (type.group !== '' &&
        !['apps', 'batch', 'autoscaling', 'policy'].includes(type.group) &&
        !type.group.endsWith('.k8s.io'))
    );
  }
  return sets[category].has(type.resource);
}

export function typeKey(type: ResourceType): string {
  return `${type.group}/${type.version}/${type.resource}`;
}

export function typeLabel(type: ResourceType): string {
  const label = resourceNames[type.resource];
  return label ? `${label} · ${type.kind}` : `${type.kind} · ${type.group || 'core'}`;
}

export function ageLabel(value: string): string {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return '—';
  const minutes = Math.max(0, Math.floor((Date.now() - timestamp) / 60000));
  if (minutes < 60) return `${minutes} 分钟`;
  if (minutes < 1440) return `${Math.floor(minutes / 60)} 小时`;
  return `${Math.floor(minutes / 1440)} 天`;
}
