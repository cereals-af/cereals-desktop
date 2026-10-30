import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Empty, Input, Select, Skeleton } from 'antd';
import { useAPI } from '../../api/context';
import type { Profile } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { categoryLabels, inCategory, typeKey, typeLabel } from './catalog';
import type { Category } from './catalog';
import { ResourceTable } from './ResourceTable';
import { NamespaceSelect } from './NamespaceSelect';

export function ResourceBrowser({ profile, category }: { profile: Profile; category: Category }) {
  const api = useAPI();
  const [selectedType, setSelectedType] = useState('');
  const [namespaceInput, setNamespaceInput] = useState(profile.namespace);
  const [labelInput, setLabelInput] = useState('');
  const [filters, setFilters] = useState({ namespace: profile.namespace, label: '' });
  const catalog = useQuery({
    queryKey: ['cluster', profile.id, 'catalog'],
    queryFn: ({ signal }) => api.discover(profile.id, signal),
    retry: false,
    staleTime: 60000,
  });
  const types = (catalog.data?.resources ?? []).filter((type) => inCategory(type, category));
  const selected =
    types.find((type) => typeKey(type) === selectedType) ??
    types.find(
      (type) =>
        type.resource ===
        (category === 'nodes'
          ? 'nodes'
          : category === 'storage'
            ? 'persistentvolumeclaims'
            : category === 'network'
              ? 'services'
              : category === 'configuration'
                ? 'configmaps'
                : 'pods'),
    ) ??
    types[0];
  const namespace = selected?.namespaced ? filters.namespace : '';

  return (
    <>
      <div className="page-heading">
        <div>
          <div className="eyebrow">CLUSTER RESOURCES</div>
          <h1>{categoryLabels[category]}</h1>
          <p>浏览集群实际暴露的 API，查看资源状态与 YAML。</p>
        </div>
      </div>
      {catalog.isError && <Alert showIcon type="error" title={errorMessage(catalog.error)} />}
      {(catalog.data?.warnings ?? []).map((warning) => (
        <Alert key={warning} showIcon type="warning" title={warning} />
      ))}
      {category === 'custom' && (
        <Alert
          type="info"
          showIcon
          title="包含 CRD 定义和扩展 API；完整列表可在资源浏览器中查找。"
        />
      )}
      {catalog.isPending ? (
        <div className="panel loading-panel">
          <Skeleton active />
        </div>
      ) : selected ? (
        <>
          <form
            className="resource-filters"
            onSubmit={(event) => {
              event.preventDefault();
              setFilters({ namespace: namespaceInput.trim(), label: labelInput.trim() });
            }}
          >
            <label className="type-filter">
              资源类型
              <Select
                aria-label="资源类型"
                showSearch
                optionFilterProp="label"
                value={typeKey(selected)}
                onChange={setSelectedType}
                options={types.map((type) => ({ value: typeKey(type), label: typeLabel(type) }))}
              />
            </label>
            <label>
              命名空间
              <NamespaceSelect
                clusterID={profile.id}
                disabled={!selected.namespaced}
                value={namespaceInput}
                onChange={(value) => {
                  setNamespaceInput(value);
                  setFilters((current) => ({ ...current, namespace: value }));
                }}
              />
            </label>
            <label>
              标签选择器
              <Input
                aria-label="标签选择器"
                value={labelInput}
                onChange={(event) => setLabelInput(event.target.value)}
                placeholder="例如 app=nginx"
                allowClear
              />
            </label>
            <Button htmlType="submit">应用筛选</Button>
          </form>
          <ResourceTable
            key={`${typeKey(selected)}|${namespace}|${filters.label}`}
            profile={profile}
            type={selected}
            namespace={namespace}
            labelSelector={filters.label}
          />
        </>
      ) : (
        !catalog.isError && (
          <div className="panel loading-panel">
            <Empty description="当前集群没有发现此分类中的资源类型" />
          </div>
        )
      )}
      <div className="resource-footnote">
        数据来自 Kubernetes API · 可按需开启实时更新 · 最终权限由集群校验
      </div>
    </>
  );
}
