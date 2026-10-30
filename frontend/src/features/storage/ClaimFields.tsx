import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Input, Select } from 'antd';
import { useAPI } from '../../api/context';
import type { ClaimForm } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { IdentityFields } from '../service-config/IdentityFields';
import { ClassCapability } from './ClassDetails';
import { listClasses } from './models';

export function ClaimFields({
  value,
  onChange,
  clusterID,
}: {
  value: ClaimForm;
  onChange: (form: ClaimForm) => void;
  clusterID: string;
}) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', clusterID, 'storage-classes'],
    queryFn: ({ signal }) => listClasses(api, clusterID, signal),
    enabled: value.classMode === 'named',
    retry: false,
    staleTime: 30000,
  });
  const names = new Set(query.data ?? []);
  if (value.className) names.add(value.className);
  return (
    <div className="storage-fields">
      <IdentityFields
        kind="PVC"
        clusterID={clusterID}
        name={value.name}
        namespace={value.namespace}
        onName={(name) => onChange({ ...value, name })}
        onNamespace={(namespace) => onChange({ ...value, namespace })}
      />
      <div className="workload-fields">
        <label>
          申请容量
          <Input
            aria-label="申请容量"
            value={value.capacity}
            placeholder="例如 10Gi"
            onChange={(event) => onChange({ ...value, capacity: event.target.value })}
          />
        </label>
        <label>
          卷模式
          <Select
            aria-label="卷模式"
            value={value.volumeMode}
            onChange={(volumeMode) => onChange({ ...value, volumeMode })}
            options={['Filesystem', 'Block'].map((v) => ({ value: v, label: v }))}
          />
        </label>
        <label>
          访问模式
          <Select
            mode="multiple"
            aria-label="访问模式"
            value={value.accessModes ?? []}
            onChange={(accessModes: string[]) => onChange({ ...value, accessModes })}
            options={['ReadWriteOnce', 'ReadOnlyMany', 'ReadWriteMany', 'ReadWriteOncePod'].map(
              (v) => ({ value: v, label: v }),
            )}
          />
        </label>
        <label>
          存储类策略
          <Select
            aria-label="存储类策略"
            value={value.classMode}
            onChange={(classMode) => onChange({ ...value, classMode, className: '' })}
            options={[
              { value: 'default', label: '使用集群默认存储类' },
              { value: 'named', label: '指定 StorageClass' },
              { value: 'none', label: '显式无存储类' },
            ]}
          />
        </label>
        {value.classMode === 'named' && (
          <label>
            StorageClass
            {query.isError ? (
              <Input
                aria-label="StorageClass 名称"
                value={value.className}
                onChange={(event) => onChange({ ...value, className: event.target.value })}
              />
            ) : (
              <Select
                aria-label="StorageClass 名称"
                showSearch
                loading={query.isFetching}
                value={value.className || undefined}
                onChange={(className) => onChange({ ...value, className })}
                options={[...names].map((v) => ({ value: v, label: v }))}
              />
            )}
            <Button size="small" onClick={() => void query.refetch()}>
              刷新 StorageClass 列表
            </Button>
          </label>
        )}
      </div>
      {value.classMode === 'named' && query.isError && (
        <Alert
          showIcon
          type="warning"
          title={errorMessage(query.error)}
          description="可手动填写存储类名称；预检仍需读取所选类。"
        />
      )}
      <Alert
        showIcon
        type="info"
        title="预检后再创建 PVC"
        description={
          value.classMode === 'default'
            ? '不写 storageClassName，由集群决定默认类；未配置默认类时可能保持 Pending。复杂选择器、静态 PV 预绑定、快照或克隆请使用 YAML 创建。'
            : value.classMode === 'none'
              ? '写入空 storageClassName，不使用默认动态供应；需要匹配无存储类的 PV。'
              : '访问模式、卷模式和扩容能力由存储驱动决定；指定存储类在正式提交前会再次核对。'
        }
      />
      {value.classMode === 'named' && value.className && (
        <ClassCapability key={value.className} clusterID={clusterID} name={value.className} />
      )}
    </div>
  );
}
