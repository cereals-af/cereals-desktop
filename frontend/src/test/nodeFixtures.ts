import { fixturePodUID } from './sessionFixtures';
import type {
  DesktopAPI,
  NodeSnapshot,
  Ref,
  DrainPlan,
  DrainProgress,
  NodePod,
  CordonRequest,
} from '../api/types';
import { mutationFixtures } from './mutationFixtures';
const snapshots = new Map<string, NodeSnapshot>();
const pending = new Map<string, CordonRequest>();
const plans = new Map<string, { target: Ref; plan: DrainPlan; timeout: number }>();
const tasks = new Map<string, { progress: DrainProgress; polls: number; timeout: number }>();
const key = (target: Ref) => `${target.clusterID}/${target.name}`;
function current(target: Ref): NodeSnapshot {
  let result = snapshots.get(key(target));
  if (!result) {
    result = {
      baseline: { uid: `node-${target.name}`, resourceVersion: '10' },
      unschedulable: false,
      roles: ['worker'],
      addresses: { InternalIP: '192.0.2.31' },
      info: {
        kubeletVersion: 'v1.36.2',
        osImage: 'Ubuntu 24.04',
        containerRuntimeVersion: 'containerd://2.1.4',
      },
      capacity: { cpu: '8', memory: '33554432Ki', pods: '110' },
      allocatable: { cpu: '7800m', memory: '31457280Ki', pods: '110' },
      labels: { 'node-role.kubernetes.io/worker': '' },
      taints: [],
      conditions: [
        {
          type: 'Ready',
          status: 'True',
          reason: 'KubeletReady',
          message: 'kubelet is ready',
          updated: '2026-10-27T12:00:00Z',
        },
      ],
    };
    snapshots.set(key(target), result);
  }
  return structuredClone(result);
}
const pods: NodePod[] = [
  {
    uid: 'node-pod-1',
    name: 'api-gateway-6f8d7c9b4-x2k5m',
    namespace: 'platform',
    phase: 'Running',
    owner: 'ReplicaSet/api-gateway',
    category: 'managed',
    localData: [],
    terminating: false,
  },
  {
    uid: 'node-pod-2',
    name: 'node-agent',
    namespace: 'kube-system',
    phase: 'Running',
    owner: 'DaemonSet/node-agent',
    category: 'daemonset',
    localData: [],
    terminating: false,
  },
  {
    uid: 'node-pod-3',
    name: 'local-cache',
    namespace: 'platform',
    phase: 'Running',
    owner: '',
    category: 'unmanaged',
    localData: ['cache (emptyDir)'],
    terminating: false,
  },
];
export function commitNode(token: string) {
  const request = pending.get(token);
  pending.delete(token);
  if (request) {
    const snapshot = current(request.target);
    snapshot.unschedulable = request.unschedulable;
    snapshot.baseline.resourceVersion = '11';
    snapshots.set(key(request.target), snapshot);
  }
}
export function discardNode(token: string) {
  pending.delete(token);
}
export function nodeRows(clusterID: string) {
  return ['worker-01'].map((name) => {
    const state = current({
      clusterID,
      group: '',
      version: 'v1',
      resource: 'nodes',
      namespace: '',
      name,
    });
    return {
      uid: `node-${name}`,
      name,
      namespace: '',
      createdAt: '2026-10-24T10:00:00Z',
      status: 'Ready',
      detail: 'worker',
      labels: {},
      node: {
        roles: 'worker',
        internalIP: '192.0.2.31',
        kubeletVersion: 'v1.36.2',
        cpu: '8',
        memory: '33554432Ki',
        unschedulable: state.unschedulable,
      },
    };
  });
}
export const nodeFixtures: Pick<
  DesktopAPI,
  | 'node'
  | 'nodePods'
  | 'previewCordon'
  | 'previewDrain'
  | 'discardDrain'
  | 'startDrain'
  | 'pollDrain'
  | 'cancelDrain'
  | 'closeDrain'
> = {
  node: (target) => Promise.resolve(current(target)),
  nodePods: (target, namespace) => {
    if (target.clusterID === 'test-staging' && !namespace)
      return Promise.reject(new Error('权限不足：无法跨命名空间列出 Pod'));
    return Promise.resolve({
      pods: pods
        .filter((pod) => !namespace || pod.namespace === namespace)
        .map((pod) => ({
          ...pod,
          uid: fixturePodUID({
            ...target,
            group: '',
            resource: 'pods',
            namespace: pod.namespace,
            name: pod.name,
          }),
        })),
      continue: '',
    });
  },
  previewCordon: async (request) => {
    const preview = await mutationFixtures.preview({
      target: request.target,
      baseline: request.baseline,
      mode: 'patch',
      revealSecret: false,
      yaml: `apiVersion: v1\nkind: Node\nmetadata:\n  name: ${request.target.name}\nspec:\n  unschedulable: ${request.unschedulable}\n`,
    });
    pending.set(preview.token, request);
    return preview;
  },
  previewDrain: async (request) => {
    if (request.target.clusterID === 'test-staging')
      throw new Error('权限不足：排空需要完整节点 Pod 清单');
    const cordon = await nodeFixtures.previewCordon({ ...request, unschedulable: true });
    discardNode(cordon.token);
    const rows = pods.map((pod) => ({
      pod,
      state:
        pod.category === 'daemonset'
          ? request.options.ignoreDaemonSets
            ? 'skipped'
            : 'blocked'
          : pod.localData?.length &&
              (!request.options.allowLocalData || !request.options.allowUnmanaged)
            ? 'blocked'
            : 'pending',
      message:
        pod.category === 'daemonset'
          ? 'DaemonSet Pod 保留项'
          : pod.localData?.length
            ? '需要确认本地数据和无控制器引用影响'
            : '等待 PDB 放行或 API 限流恢复',
    }));
    const plan = {
      token: rows.some((row) => row.state === 'blocked') ? '' : crypto.randomUUID(),
      cordon: { ...cordon, token: '' },
      pods: rows,
      expiresAt: cordon.expiresAt,
    };
    if (plan.token)
      plans.set(plan.token, {
        target: request.target,
        plan,
        timeout: request.options.timeoutSeconds,
      });
    return plan;
  },
  discardDrain: (token) => {
    plans.delete(token);
    return Promise.resolve();
  },
  startDrain: (token, confirmation) => {
    const saved = plans.get(token);
    if (!saved || confirmation !== saved.target.name)
      return Promise.reject(new Error('预检已失效或节点名称不匹配'));
    plans.delete(token);
    const id = crypto.randomUUID();
    tasks.set(id, {
      progress: {
        target: saved.target,
        state: 'running',
        message: '等待 PDB 放行或 API 限流恢复',
        pods: saved.plan.pods,
        done: false,
      },
      polls: 0,
      timeout: saved.timeout,
    });
    const state = current(saved.target);
    state.unschedulable = true;
    snapshots.set(key(saved.target), state);
    return Promise.resolve(id);
  },
  pollDrain: (id) => {
    const task = tasks.get(id);
    if (!task) return Promise.reject(new Error('排空任务不存在'));
    task.polls++;
    if (task.polls >= 4 && !task.progress.done) {
      task.progress.done = true;
      task.progress.state = task.timeout === 30 ? 'timeout' : 'completed';
      task.progress.message =
        task.timeout === 30
          ? '排空超时；节点不会自动解封。'
          : '已确认的可驱逐 Pod 均已离开；节点保持封锁。';
      if (task.timeout !== 30)
        task.progress.pods = (task.progress.pods ?? []).map((row) =>
          row.state === 'skipped' ? row : { ...row, state: 'done', message: '原 Pod UID 已不存在' },
        );
    }
    return Promise.resolve(structuredClone(task.progress));
  },
  cancelDrain: (id) => {
    const task = tasks.get(id);
    if (task) {
      task.progress.done = true;
      task.progress.state = 'cancelled';
      task.progress.message = '已停止后续操作；已接受的驱逐不会回滚。';
    }
    return Promise.resolve();
  },
  closeDrain: (id) => {
    tasks.delete(id);
    return Promise.resolve();
  },
};
