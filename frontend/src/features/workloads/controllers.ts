import type { ControllerForm, Ref } from '../../api/types';
import { emptyDeployment } from './defaults';

export function controllerKind(target: Pick<Ref, 'group' | 'version' | 'resource'>) {
  if (target.group !== 'apps' || target.version !== 'v1') return '';
  switch (target.resource) {
    case 'statefulsets':
      return 'StatefulSet';
    case 'daemonsets':
      return 'DaemonSet';
    case 'replicasets':
      return 'ReplicaSet';
    default:
      return '';
  }
}
export function emptyController(namespace: string): ControllerForm {
  return {
    workload: emptyDeployment(namespace),
    strategy: 'RollingUpdate',
    partition: 0,
    maxUnavailable: '1',
    maxSurge: '0',
    serviceName: '',
    podManagementPolicy: 'OrderedReady',
    retainDeleted: 'Retain',
    retainScaled: 'Retain',
    nodeSelector: [],
    claims: [],
  };
}
export function strategyNote(resource: string, form: ControllerForm): string {
  if (resource === 'replicasets')
    return 'ReplicaSet 模板变更只影响后续新建 Pod，不会滚动替换已有 Pod；由 Deployment 管理时，优先进入所属 Deployment 操作。';
  if (!['RollingUpdate', 'OnDelete'].includes(form.strategy))
    return `当前更新策略为 ${form.strategy}。此表单保留该策略；高级策略请使用 YAML 编辑，实际 Pod 替换行为由集群控制器决定。`;
  if (form.strategy === 'OnDelete')
    return 'OnDelete：模板更新、重启和回退只改变模板。已有 Pod 不会自动替换，需要逐个确认并删除旧 Pod，由控制器按新模板补建。';
  if (resource === 'statefulsets')
    return `RollingUpdate：按 StatefulSet 顺序更新，partition = ${form.partition}；仅更新分区内的 Pod（起始序号非零时以集群实际序号规则为准）。分区外 Pod 继续使用旧版本。回退后若更新因未就绪 Pod 卡住，可能仍需手动删除失败 Pod。`;
  return `RollingUpdate：maxUnavailable = ${form.maxUnavailable}，maxSurge = ${form.maxSurge}；由节点匹配条件决定期望数量。`;
}
