import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Space, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';

export function WorkloadAccess({ target, scale = false }: { target: Ref; scale?: boolean }) {
  const access = useWorkloadAccess(target);
  const checks = (access.data ?? []).filter((item) =>
    scale ? item.subresource === 'scale' : item.subresource === '',
  );
  return (
    <div className="workload-access">
      <Space wrap>
        <span className="muted">{scale ? `${target.resource}/scale` : target.resource} 权限</span>
        {access.isPending && <Tag>检查中</Tag>}
        {checks.map((item) => (
          <Tag
            key={item.verb}
            color={item.state === 'allowed' ? 'green' : item.state === 'denied' ? 'red' : 'default'}
            title={item.message}
          >
            {item.verb} ·{' '}
            {item.state === 'allowed' ? '允许' : item.state === 'denied' ? '拒绝' : '未知'}
          </Tag>
        ))}
        <Button size="small" loading={access.isFetching} onClick={() => void access.refetch()}>
          刷新权限
        </Button>
      </Space>
      {access.isError && (
        <Alert type="warning" title={`权限提示不可用：${errorMessage(access.error)}`} />
      )}
      {checks.some((item) => item.state === 'unknown') && (
        <p className="muted">权限检查未完成，可尝试预检；最终以 API Server 授权结果为准。</p>
      )}
    </div>
  );
}

function useWorkloadAccess(target: Ref) {
  const api = useAPI();
  return useQuery({
    queryKey: ['cluster', target.clusterID, 'workload-permissions', target],
    queryFn: ({ signal }) => api.workloadPermissions(target, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
}
