import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Descriptions, Spin, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { StorageClass } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { classRef } from './models';

export function ClassDetails({ value }: { value: StorageClass }) {
  return (
    <>
      <Descriptions
        bordered
        column={2}
        size="small"
        items={[
          { key: 'driver', label: 'Provisioner', children: value.provisioner },
          { key: 'default', label: '默认存储类', children: value.default ? '是' : '否' },
          { key: 'policy', label: '回收策略', children: value.reclaimPolicy },
          { key: 'binding', label: '绑定模式', children: value.bindingMode },
          {
            key: 'expand',
            label: '允许扩容',
            children: (
              <Tag color={value.allowExpansion ? 'green' : 'orange'}>
                {value.allowExpansion ? '允许申请扩容' : '未允许扩容'}
              </Tag>
            ),
          },
          { key: 'mount', label: '挂载选项', children: value.mountOptions?.join(', ') || '—' },
        ]}
      />
      {value.bindingMode === 'WaitForFirstConsumer' && (
        <Alert
          showIcon
          type="info"
          title="等待首个使用者"
          description="PVC 可能保持 Pending，直到使用它的 Pod 参与调度；结合 Pod 和存储事件判断是否异常。"
        />
      )}
      <p className="muted">
        扩容仍取决于存储驱动、配额和控制器；回收策略 Delete 可能删除后端存储。驱动参数请查看 YAML。
      </p>
      {value.allowedTopologies && (
        <>
          <h3>允许的拓扑</h3>
          <pre className="storage-json">{value.allowedTopologies}</pre>
        </>
      )}
    </>
  );
}

export function ClassCapability({ clusterID, name }: { clusterID: string; name: string }) {
  const api = useAPI();
  const target = classRef(clusterID, name);
  const query = useQuery({
    queryKey: ['cluster', clusterID, 'storage-class', name],
    queryFn: ({ signal }) => api.storageClass(target, signal),
    gcTime: 0,
    retry: false,
  });
  return (
    <section className="storage-section">
      <h3>StorageClass · {name}</h3>
      <Button size="small" loading={query.isFetching} onClick={() => void query.refetch()}>
        刷新存储类
      </Button>
      {query.isPending && <Spin />}
      {query.isError && (
        <Alert
          showIcon
          type="warning"
          title={errorMessage(query.error)}
          description="存储类能力暂不可确认；PVC/PV 详情仍可查看。扩容预检需要读取该 StorageClass。"
        />
      )}
      {query.data && <ClassDetails value={query.data} />}
    </section>
  );
}
