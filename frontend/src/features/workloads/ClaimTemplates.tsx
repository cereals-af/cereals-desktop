import { Button, Input, Select, Table } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useAPI } from '../../api/context';
import type { ClaimTemplate } from '../../api/types';
import { listClasses } from '../storage/models';

export function ClaimTemplates({
  clusterID,
  value,
  creating,
  onChange,
}: {
  clusterID: string;
  value: ClaimTemplate[];
  creating: boolean;
  onChange: (claims: ClaimTemplate[]) => void;
}) {
  const api = useAPI();
  const classes = useQuery({
    queryKey: ['cluster', clusterID, 'storage-classes'],
    queryFn: ({ signal }) => listClasses(api, clusterID, signal),
    enabled: creating && value.some((v) => v.classMode === 'named'),
    retry: false,
  });
  if (!creating)
    return (
      <Table
        size="small"
        rowKey="name"
        pagination={false}
        dataSource={value}
        locale={{ emptyText: '没有 PVC 模板' }}
        columns={[
          { title: '模板', dataIndex: 'name' },
          { title: '容量', dataIndex: 'capacity' },
          {
            title: 'StorageClass',
            render: (_, c) =>
              c.classMode === 'named'
                ? c.className
                : c.classMode === 'none'
                  ? '无存储类'
                  : '集群默认',
          },
          { title: '访问模式', render: (_, c) => c.accessModes?.join(', ') },
          { title: '卷模式', dataIndex: 'volumeMode' },
        ]}
      />
    );
  const set = (index: number, patch: Partial<ClaimTemplate>) =>
    onChange(value.map((v, i) => (i === index ? { ...v, ...patch } : v)));
  return (
    <>
      {value.map((claim, index) => (
        <section className="operation-card" key={index}>
          <h3>PVC 模板 {index + 1}</h3>
          <div className="workload-fields">
            <label>
              模板名称
              <Input
                aria-label={`PVC 模板名称 ${index + 1}`}
                value={claim.name}
                onChange={(e) => set(index, { name: e.target.value })}
              />
            </label>
            <label>
              容量
              <Input
                aria-label={`PVC 容量 ${index + 1}`}
                value={claim.capacity}
                onChange={(e) => set(index, { capacity: e.target.value })}
              />
            </label>
            <label>
              挂载路径
              <Input
                aria-label={`PVC 挂载路径 ${index + 1}`}
                value={claim.mountPath}
                onChange={(e) => set(index, { mountPath: e.target.value })}
              />
            </label>
            <label>
              访问模式
              <Select
                mode="multiple"
                aria-label={`PVC 访问模式 ${index + 1}`}
                value={claim.accessModes}
                onChange={(accessModes: string[]) => set(index, { accessModes })}
                options={['ReadWriteOnce', 'ReadWriteMany', 'ReadOnlyMany', 'ReadWriteOncePod'].map(
                  (v) => ({ value: v, label: v }),
                )}
              />
            </label>
            <label>
              存储类策略
              <Select
                aria-label={`PVC 存储类策略 ${index + 1}`}
                value={claim.classMode}
                onChange={(classMode) => set(index, { classMode, className: '' })}
                options={[
                  { value: 'default', label: '集群默认' },
                  { value: 'named', label: '指定 StorageClass' },
                  { value: 'none', label: '无存储类' },
                ]}
              />
            </label>
            {claim.classMode === 'named' && (
              <label>
                StorageClass
                {classes.isError ? (
                  <Input
                    aria-label={`PVC StorageClass ${index + 1}`}
                    value={claim.className}
                    onChange={(e) => set(index, { className: e.target.value })}
                  />
                ) : (
                  <Select
                    aria-label={`PVC StorageClass ${index + 1}`}
                    showSearch
                    loading={classes.isFetching}
                    value={claim.className || undefined}
                    options={[
                      ...new Set([
                        ...(classes.data ?? []),
                        ...(claim.className ? [claim.className] : []),
                      ]),
                    ].map((v) => ({ value: v, label: v }))}
                    onChange={(className) => set(index, { className })}
                  />
                )}
              </label>
            )}
          </div>
          {classes.isError && <p className="muted">StorageClass 列表读取失败，可手动填写名称。</p>}
          <p className="muted">
            Filesystem 卷挂载到每个常规容器；Block 卷、数据源或特殊挂载请使用 YAML。
          </p>
          <Button onClick={() => onChange(value.filter((_, i) => i !== index))}>
            移除此 PVC 模板
          </Button>
        </section>
      ))}
      <Button
        onClick={() =>
          onChange([
            ...value,
            {
              name: `data-${value.length + 1}`,
              capacity: '10Gi',
              classMode: 'default',
              className: '',
              accessModes: ['ReadWriteOnce'],
              volumeMode: 'Filesystem',
              mountPath: `/data-${value.length + 1}`,
            },
          ])
        }
      >
        添加 PVC 模板
      </Button>
    </>
  );
}
