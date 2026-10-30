import { useQuery } from '@tanstack/react-query';
import { Alert } from 'antd';
import { useAPI } from '../../api/context';
import type { Ref } from '../../api/types';

export function ScaleStorageNotice({ target, replicas }: { target: Ref; replicas: number }) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'scale-storage', target],
    queryFn: ({ signal }) => api.controller(target, signal),
    enabled: target.resource === 'statefulsets',
    retry: false,
    gcTime: 0,
  });
  if (target.resource !== 'statefulsets') return null;
  if (!query.data)
    return (
      <Alert
        type="warning"
        showIcon
        title={
          query.isPending
            ? '正在读取 StatefulSet 的 PVC 保留策略…'
            : '无法读取 StatefulSet 的 PVC 保留策略'
        }
        description="scale 权限与父资源读取权限独立。缩容前请确认 whenScaled 策略；Delete 可能删除被缩减实例的 PVC，进而触发 PV 数据回收。"
      />
    );
  const shrinking = replicas < query.data.form.workload.replicas;
  return (
    <Alert
      showIcon
      type={shrinking ? 'warning' : 'info'}
      title={`${shrinking ? '本次缩容' : '缩容'} PVC 策略：${query.data.form.retainScaled}`}
      description={
        query.data.form.retainScaled === 'Delete'
          ? '控制器会删除被缩减实例对应且符合策略的 PVC。底层卷是否删除取决于 PV 回收策略；正式提交前请确认差异和数据保留要求。'
          : '被缩减实例的 PVC 会保留，后续扩容可能重新使用；手工删除 PVC 或外部自动化仍可能影响数据。'
      }
    />
  );
}
