import { useState } from 'react';
import { UsageCell } from '../metrics/UsageCell';
import { usageKey, useResourceUsage } from '../metrics/useResourceUsage';
import { useTableColumns } from './useTableColumns';
import { TableColumnSettings } from './TableColumnSettings';
import { VisualWorkspace } from '../visual-forms/VisualWorkspace';
import { visualKind } from '../visual-forms/models';
import { NodeWorkspace } from '../nodes/NodeWorkspace';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Input, Space, Switch, Table, Tooltip } from 'antd';
import { ChevronLeft, ChevronRight, RefreshCw, Search } from 'lucide-react';
import { useAPI } from '../../api/context';
import type { Item, Profile, Ref, ResourceType } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { typeKey } from './catalog';
import { resourceColumns } from './resourceColumns';
import { ManifestViewer } from './ManifestViewer';
import { ManifestEditor } from './ManifestEditor';
import { DeleteResource } from './DeleteResource';
import { useResourceWatch } from './useResourceWatch';
import { ControllerWorkspace } from '../workloads/ControllerWorkspace';
import { controllerKind } from '../workloads/controllers';
import { DeploymentWorkspace } from '../workloads/DeploymentWorkspace';
import { ScaleWorkspace } from '../workloads/ScaleWorkspace';
import { supportsScale } from '../workloads/scale';
import { CoreWorkspace } from '../service-config/CoreWorkspace';
import { CRDWorkspace } from '../custom-resources/CRDWorkspace';
import { coreFormKind } from '../service-config/models';
import { isStorage } from '../storage/models';
import { StorageWorkspace } from '../storage/StorageWorkspace';

interface Props {
  profile: Profile;
  type: ResourceType;
  namespace: string;
  labelSelector: string;
}

export function ResourceTable({ profile, type, namespace, labelSelector }: Props) {
  const api = useAPI();
  const cache = useQueryClient();
  const [cursors, setCursors] = useState(['']);
  const [search, setSearch] = useState('');
  const [custom, setCustom] = useState<Ref | null>(null);
  const isDefinition =
    type.group === 'apiextensions.k8s.io' && type.resource === 'customresourcedefinitions';
  const [node, setNode] = useState<Ref | null>(null);
  const [storage, setStorage] = useState<Ref | null>(null);
  const storageType = isStorage(type);
  const isClaim = storageType && type.resource === 'persistentvolumeclaims';
  const isNode = type.group === '' && type.resource === 'nodes' && type.version === 'v1';
  const [scale, setScale] = useState<Ref | null>(null);
  const [selected, setSelected] = useState<Ref | null>(null);
  const [visual, setVisual] = useState<Ref | null>(null);
  const isDeployment =
    type.group === 'apps' && type.version === 'v1' && type.resource === 'deployments';
  const isController = Boolean(controllerKind(type));
  const [initialWorkloadTab, setInitialWorkloadTab] = useState('status');
  const formKind = coreFormKind(type);
  const builtinKind = visualKind(type);
  const isBatch = builtinKind === 'Job' || builtinKind === 'CronJob';
  const isVisual = isDeployment || isController || Boolean(formKind) || Boolean(builtinKind);
  const [action, setAction] = useState<{ kind: 'edit' | 'delete'; target: Ref } | null>(null);
  const [live, setLive] = useState(false);
  const cursor = cursors.at(-1) ?? '';
  const scope = {
    clusterID: profile.id,
    group: type.group,
    version: type.version,
    resource: type.resource,
    namespace,
    labelSelector,
    continue: cursor,
  };
  const query = useQuery({
    queryKey: ['cluster', profile.id, 'list', typeKey(type), namespace, labelSelector, cursor],
    queryFn: ({ signal }) =>
      api.list(
        {
          clusterID: profile.id,
          group: type.group,
          version: type.version,
          resource: type.resource,
          namespace,
          labelSelector,
          continue: cursor,
        },
        signal,
      ),
    retry: false,
  });
  const refresh = () => {
    setCursors(['']);
    void cache.invalidateQueries({ queryKey: ['cluster', profile.id, 'list', typeKey(type)] });
  };
  const watch = useResourceWatch(scope, query.data?.resourceVersion, live, refresh);
  const items = query.data?.items ?? [];
  const metricsEnabled = type.group === '' && ['pods', 'nodes'].includes(type.resource);
  const columnSettings = useTableColumns(typeKey(type));
  const {
    host: usageHost,
    samples: usageSamples,
    reason: usageReason,
    refresh: refreshUsage,
  } = useResourceUsage(
    {
      clusterID: profile.id,
      resource: type.resource,
      namespace,
      targets: metricsEnabled
        ? items.map(({ name, namespace, uid }) => ({ name, namespace, uid }))
        : [],
    },
    metricsEnabled && columnSettings.shown('usage'),
  );
  const filtered = items.filter((item) => item.name.toLowerCase().includes(search.toLowerCase()));
  const itemTarget = (item: Item): Ref => ({
    clusterID: profile.id,
    group: type.group,
    version: type.version,
    resource: type.resource,
    namespace: item.namespace,
    name: item.name,
  });
  const openVisual = (target: Ref) => {
    setInitialWorkloadTab('status');
    setVisual(target);
  };
  const columns = resourceColumns(type, (item) =>
    (storageType
      ? setStorage
      : isNode
        ? setNode
        : isDefinition
          ? setCustom
          : isVisual
            ? openVisual
            : setSelected)(itemTarget(item)),
  );
  if (metricsEnabled) {
    const index = columns.findIndex(
      (column) => 'dataIndex' in column && column.dataIndex === 'createdAt',
    );
    columns.splice(index, 0, {
      title: 'CPU / 内存用量',
      key: 'usage',
      width: 170,
      render: (_, item) => (
        <UsageCell sample={usageSamples.get(usageKey(item))} reason={usageReason} />
      ),
    });
  }
  if (supportsScale(type))
    columns.push({
      title: '操作',
      key: 'scale',
      width: 100,
      render: (_, item) => (
        <Button
          size="small"
          aria-label={`扩缩容 ${item.name}`}
          onClick={() => setScale(itemTarget(item))}
        >
          扩缩容
        </Button>
      ),
    });
  if (isDeployment || isController || isBatch)
    columns.push({
      title: 'Pod 操作',
      key: 'pod-actions',
      width: 130,
      render: (_, item) => (
        <Space>
          <Button
            size="small"
            aria-label={`日志 ${item.name}`}
            onClick={() => {
              setInitialWorkloadTab('pods');
              setVisual(itemTarget(item));
            }}
          >
            日志
          </Button>
          <Button
            size="small"
            aria-label={`终端 ${item.name}`}
            onClick={() => {
              setInitialWorkloadTab('pods');
              setVisual(itemTarget(item));
            }}
          >
            终端
          </Button>
        </Space>
      ),
    });
  return (
    <section ref={usageHost} className="panel resource-panel">
      <div className="table-heading">
        <div>
          <h2>
            {type.kind}
            <span className="count-badge">{items.length}</span>
          </h2>
          <span className="muted">
            {type.group || 'core'} / {type.version}
          </span>
        </div>
        <div className="table-actions">
          {type.group &&
            !storageType &&
            !isDefinition &&
            !['apps', 'batch', 'autoscaling', 'policy', 'networking.k8s.io'].includes(
              type.group,
            ) && <Button onClick={() => setCustom({ ...scope, name: '' })}>CRD 定义与实例</Button>}
          {supportsScale(type) && (
            <Button
              onClick={() =>
                setScale({ ...scope, namespace: namespace || profile.namespace, name: '' })
              }
            >
              按名称扩缩容
            </Button>
          )}
          {(isVisual || isClaim) && type.verbs?.includes('create') && (
            <Button
              type="primary"
              onClick={() =>
                (isClaim ? setStorage : openVisual)({
                  ...scope,
                  namespace: namespace || profile.namespace,
                  name: '',
                })
              }
            >
              创建 {isClaim ? 'PVC' : type.kind}
            </Button>
          )}
          {type.verbs?.includes('create') && (
            <Button
              type={isVisual || isClaim ? 'default' : 'primary'}
              onClick={() =>
                setAction({
                  kind: 'edit',
                  target: {
                    ...scope,
                    namespace: type.namespaced ? namespace || profile.namespace : '',
                    name: '',
                  },
                })
              }
            >
              {isVisual || isClaim ? 'YAML 创建' : '创建资源'}
            </Button>
          )}
          <Input
            aria-label="搜索本页资源"
            placeholder="搜索本页资源名称"
            value={search}
            prefix={<Search size={15} />}
            onChange={(event) => setSearch(event.target.value)}
            allowClear
          />
          <Tooltip title="重新加载第一页">
            <Button
              aria-label="刷新资源"
              icon={<RefreshCw size={15} />}
              loading={query.isFetching}
              onClick={() => {
                if (metricsEnabled && columnSettings.shown('usage')) void refreshUsage();
                if (cursors.length > 1) setCursors(['']);
                else void query.refetch();
              }}
            />
          </Tooltip>
          {metricsEnabled && (
            <TableColumnSettings
              settings={columnSettings}
              options={[
                { key: 'usage', label: 'CPU / 内存用量' },
                { key: 'createdAt', label: '创建时间' },
                ...(isNode
                  ? [
                      { key: 'version', label: 'Kubelet' },
                      { key: 'node', label: '角色 / IP' },
                    ]
                  : [{ key: 'namespace', label: '命名空间' }]),
              ]}
            />
          )}
        </div>
      </div>
      <div className="watch-toolbar">
        <Switch
          aria-label="实时更新"
          checked={live}
          disabled={!type.verbs?.includes('watch')}
          onChange={setLive}
        />
        <span>
          {!live
            ? '手动刷新'
            : watch.state === 'live'
              ? '实时更新中'
              : watch.state === 'error'
                ? '实时更新已停止'
                : '正在连接 / 重连'}
        </span>
        {live && <span className="muted">资源变化后刷新第一页</span>}
      </div>
      {live && watch.message && (
        <Alert showIcon type={watch.state === 'error' ? 'warning' : 'info'} title={watch.message} />
      )}
      {query.isError && (
        <Alert
          className="table-error"
          showIcon
          type="error"
          title={errorMessage(query.error)}
          action={
            <Button
              size="small"
              onClick={() => {
                setCursors(['']);
                void query.refetch();
              }}
            >
              重试
            </Button>
          }
        />
      )}
      <Table<Item>
        columns={columns.filter((column) => {
          const key = column.key ?? ('dataIndex' in column ? column.dataIndex : undefined);
          return typeof key !== 'string' || columnSettings.shown(key);
        })}
        dataSource={filtered}
        rowKey={(item) => item.uid || `${item.namespace}/${item.name}`}
        loading={query.isFetching}
        pagination={false}
        scroll={{ x: 850 }}
        locale={{ emptyText: query.isError ? '资源加载失败' : '当前范围内没有资源' }}
      />
      <div className="table-footer">
        <span>本页 {items.length} 项 · 每页最多 100 项 · 名称搜索仅作用于本页</span>
        <div>
          <Button
            aria-label="上一页"
            icon={<ChevronLeft size={15} />}
            disabled={cursors.length === 1 || query.isFetching}
            onClick={() => setCursors((current) => current.slice(0, -1))}
          />
          <span>第 {cursors.length} 页</span>
          <Button
            aria-label="下一页"
            icon={<ChevronRight size={15} />}
            disabled={!query.data?.continue || query.isFetching}
            onClick={() => {
              const next = query.data?.continue;
              if (next) setCursors((current) => [...current, next]);
            }}
          />
        </div>
      </div>
      {node && (
        <NodeWorkspace
          target={node}
          profile={profile}
          onClose={() => setNode(null)}
          onSuccess={refresh}
        />
      )}
      {storage && (
        <StorageWorkspace
          target={storage}
          profile={profile}
          canPatch={Boolean(type.verbs?.includes('patch'))}
          onClose={() => setStorage(null)}
          onSuccess={refresh}
          onYAML={(target) => {
            setStorage(null);
            setSelected(target);
          }}
        />
      )}
      {custom && (
        <CRDWorkspace
          target={custom}
          profile={profile}
          onClose={() => setCustom(null)}
          onSuccess={refresh}
        />
      )}
      {scale && (
        <ScaleWorkspace
          target={scale}
          profile={profile}
          onClose={() => setScale(null)}
          onSuccess={refresh}
        />
      )}
      {selected && (
        <ManifestViewer
          resource={selected}
          contextName={profile.context}
          onClose={() => setSelected(null)}
          onEdit={
            type.verbs?.includes('patch')
              ? () => {
                  setAction({ kind: 'edit', target: selected });
                  setSelected(null);
                }
              : undefined
          }
          onDelete={
            type.verbs?.includes('delete')
              ? () => {
                  setAction({ kind: 'delete', target: selected });
                  setSelected(null);
                }
              : undefined
          }
        />
      )}
      {visual && builtinKind && (
        <VisualWorkspace
          key={JSON.stringify(visual)}
          target={visual}
          profile={profile}
          canPatch={Boolean(type.verbs?.includes('patch'))}
          canDelete={Boolean(type.verbs?.includes('delete'))}
          initialTab={initialWorkloadTab}
          onClose={() => setVisual(null)}
          onSuccess={refresh}
          onYAML={(target) => {
            setVisual(null);
            setSelected(target);
          }}
        />
      )}
      {visual && isController && (
        <ControllerWorkspace
          key={JSON.stringify(visual)}
          target={visual}
          profile={profile}
          initialTab={initialWorkloadTab}
          canPatch={Boolean(type.verbs?.includes('patch'))}
          onClose={() => setVisual(null)}
          onSuccess={refresh}
          onYAML={setSelected}
        />
      )}
      {visual && isDeployment && (
        <DeploymentWorkspace
          target={visual}
          initialTab={initialWorkloadTab}
          profile={profile}
          canPatch={Boolean(type.verbs?.includes('patch'))}
          onClose={() => setVisual(null)}
          onSuccess={refresh}
          onYAML={(target) => {
            setSelected(target);
          }}
        />
      )}
      {visual && formKind && (
        <CoreWorkspace
          target={visual}
          kind={formKind}
          profile={profile}
          canPatch={Boolean(type.verbs?.includes('patch'))}
          onClose={() => setVisual(null)}
          onSuccess={refresh}
          onYAML={(target) => {
            setVisual(null);
            setSelected(target);
          }}
        />
      )}
      {action?.kind === 'edit' && (
        <ManifestEditor
          target={action.target}
          profile={profile}
          type={type}
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
    </section>
  );
}
