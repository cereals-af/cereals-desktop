import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Checkbox, Input, Space, Table } from 'antd';
import type { TableColumnsType } from 'antd';
import { useAPI } from '../../api/context';
import type { CustomDefinition, CustomRow, Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { NamespaceSelect } from '../resources/NamespaceSelect';
import { ManifestViewer } from '../resources/ManifestViewer';
import { ManifestEditor } from '../resources/ManifestEditor';
import { DeleteResource } from '../resources/DeleteResource';
import { ResourceTable } from '../resources/ResourceTable';
import { CustomEditor } from './CustomEditor';

export function CustomInstances({
  profile,
  definition,
  version,
}: {
  profile: Profile;
  definition: CustomDefinition;
  version: string;
}) {
  const api = useAPI();
  const [namespace, setNamespace] = useState(definition.namespaced ? profile.namespace : '');
  const [label, setLabel] = useState('');
  const [filter, setFilter] = useState('');
  const [cursors, setCursors] = useState(['']);
  const [wide, setWide] = useState(false);
  const [fallback, setFallback] = useState(false);
  const [action, setAction] = useState<{
    kind: 'form' | 'yaml' | 'edit' | 'delete';
    target: Ref;
  } | null>(null);
  const scope = {
    clusterID: profile.id,
    group: definition.group,
    version,
    resource: definition.resource,
    namespace,
  };
  const type = {
    group: definition.group,
    version,
    resource: definition.resource,
    kind: definition.kind,
    namespaced: definition.namespaced,
    verbs: ['get', 'list', 'create', 'patch', 'delete'],
  };
  const query = useQuery({
    queryKey: ['cluster', profile.id, 'custom-list', scope, filter, cursors.at(-1)],
    queryFn: ({ signal }) =>
      api.customList({ ...scope, labelSelector: filter, continue: cursors.at(-1) ?? '' }, signal),
    gcTime: 0,
    retry: false,
    enabled: !fallback,
  });
  const refresh = () => {
    if (cursors.length > 1) setCursors(['']);
    else void query.refetch();
  };
  const target = (row: CustomRow): Ref => ({ ...scope, namespace: row.namespace, name: row.name });
  const createTarget: Ref = {
    ...scope,
    namespace: definition.namespaced ? namespace || profile.namespace : '',
    name: '',
  };
  const columns: TableColumnsType<CustomRow> = (query.data?.columns ?? []).flatMap(
    (column, index) =>
      column.priority > 0 && !wide
        ? []
        : [
            {
              key: `column-${index}`,
              title: column.name,
              render: (_, row) => (
                <span title={row.cells?.[index]}>{row.cells?.[index] ?? '—'}</span>
              ),
            },
          ],
  );
  if (definition.namespaced)
    columns.push({ key: 'namespace', title: '命名空间', dataIndex: 'namespace' });
  columns.push({
    key: 'actions',
    title: '操作',
    width: 230,
    render: (_, row) => (
      <Space>
        <Button
          size="small"
          aria-label={`表单编辑 ${row.name}`}
          onClick={() => setAction({ kind: 'form', target: target(row) })}
        >
          表单编辑
        </Button>
        <Button
          size="small"
          aria-label={`实例详情 ${row.name}`}
          onClick={() => setAction({ kind: 'yaml', target: target(row) })}
        >
          详情 / YAML
        </Button>
      </Space>
    ),
  });
  return (
    <div className="crd-instances">
      <div className="crd-filters">
        {definition.namespaced && (
          <NamespaceSelect
            clusterID={profile.id}
            value={namespace}
            onChange={(next) => {
              setNamespace(next);
              setCursors(['']);
            }}
          />
        )}
        <Input
          aria-label="实例标签选择器"
          placeholder="标签选择器，例如 app=demo"
          value={label}
          onChange={(event) => setLabel(event.target.value)}
        />
        <Button
          onClick={() => {
            setFilter(label);
            setCursors(['']);
          }}
        >
          筛选实例
        </Button>
        <Button onClick={refresh} loading={query.isFetching}>
          刷新实例
        </Button>
      </div>
      <Space wrap className="mutation-toolbar">
        <Button type="primary" onClick={() => setAction({ kind: 'form', target: createTarget })}>
          表单创建实例
        </Button>
        <Button onClick={() => setAction({ kind: 'edit', target: createTarget })}>
          YAML 创建实例
        </Button>
        <Checkbox checked={wide} onChange={(event) => setWide(event.target.checked)}>
          显示扩展列
        </Checkbox>
        <Checkbox checked={fallback} onChange={(event) => setFallback(event.target.checked)}>
          使用通用列表
        </Checkbox>
      </Space>
      {query.isError && !fallback && (
        <Alert
          type="warning"
          showIcon
          title={errorMessage(query.error)}
          description="可切换通用列表继续使用 YAML；CRD 定义权限不会授予实例权限。"
        />
      )}
      {fallback ? (
        <ResourceTable
          key={`${namespace}/${filter}`}
          profile={profile}
          type={type}
          namespace={namespace}
          labelSelector={filter}
        />
      ) : (
        <>
          <Table<CustomRow>
            rowKey="uid"
            dataSource={query.data?.rows ?? []}
            loading={query.isFetching}
            columns={columns}
            pagination={false}
            scroll={{ x: 850 }}
            locale={{ emptyText: query.isError ? '实例读取失败' : '当前范围没有实例' }}
          />
          <div className="table-footer">
            <span>服务端 Table · 每页最多 100 项 · 当前第 {cursors.length} 页</span>
            <Space>
              <Button
                disabled={cursors.length === 1 || query.isFetching}
                onClick={() => setCursors((all) => all.slice(0, -1))}
              >
                上一页实例
              </Button>
              <Button
                disabled={!query.data?.continue || query.isFetching}
                onClick={() => {
                  if (query.data?.continue) setCursors((all) => [...all, query.data.continue]);
                }}
              >
                下一页实例
              </Button>
            </Space>
          </div>
        </>
      )}
      {action?.kind === 'form' && (
        <CustomEditor
          target={action.target}
          profile={profile}
          namespaced={definition.namespaced}
          onClose={() => setAction(null)}
          onSuccess={refresh}
        />
      )}
      {action?.kind === 'yaml' && (
        <ManifestViewer
          resource={action.target}
          contextName={profile.context}
          onClose={() => setAction(null)}
          onEdit={() => setAction({ ...action, kind: 'edit' })}
          onDelete={() => setAction({ ...action, kind: 'delete' })}
        />
      )}
      {action?.kind === 'edit' && (
        <ManifestEditor
          target={action.target}
          type={type}
          profile={profile}
          onClose={() => setAction(null)}
          onSuccess={refresh}
        />
      )}
      {action?.kind === 'delete' && (
        <DeleteResource
          target={action.target}
          profile={profile}
          onClose={() => setAction(null)}
          onSuccess={refresh}
        />
      )}
    </div>
  );
}
