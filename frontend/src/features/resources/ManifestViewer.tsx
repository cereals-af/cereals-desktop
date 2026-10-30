import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Drawer, Space, Spin, Tabs } from 'antd';
import { RelationsPanel } from '../relations/RelationsPanel';
import { useAPI } from '../../api/context';
import type { Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { lazy, Suspense } from 'react';

const PodOperations = lazy(() =>
  import('../pods/PodOperations').then((module) => ({ default: module.PodOperations })),
);

const YAMLViewer = lazy(() => import('./YAMLViewer'));

export function ManifestViewer({
  resource,
  contextName,
  onClose,
  onEdit,
  onDelete,
  expectedUID,
  initialTab,
}: {
  expectedUID?: string;
  initialTab?: string;
  resource: Ref;
  contextName: string;
  onClose: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
}) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', resource.clusterID, 'yaml', resource, expectedUID],
    queryFn: async ({ signal }) => {
      if (!expectedUID) return api.yaml(resource, signal);
      const snapshot = await api.snapshot(resource, false, signal);
      if (snapshot.baseline.uid !== expectedUID)
        throw new Error('资源 UID 已变化，请刷新后重新选择');
      return snapshot.yaml;
    },
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  return (
    <Drawer
      open
      title={
        <div className="drawer-title">
          <span>{resource.name}</span>
          <small>
            {contextName} / {resource.namespace || '集群作用域'}
          </small>
        </div>
      }
      size={resource.group === '' && resource.resource === 'pods' ? 1160 : 760}
      onClose={onClose}
      destroyOnHidden
    >
      <Space className="mutation-toolbar">
        {onEdit && (
          <Button type="primary" onClick={onEdit}>
            编辑 YAML
          </Button>
        )}
        {onDelete && (
          <Button danger onClick={onDelete}>
            删除资源
          </Button>
        )}
      </Space>
      <Tabs
        defaultActiveKey={initialTab}
        items={[
          {
            key: 'yaml',
            label: 'YAML',
            children: (
              <>
                <Alert
                  type="info"
                  showIcon
                  title={
                    resource.group === '' && resource.resource === 'secrets'
                      ? '只读预览 · Secret 数据和注解已隐藏，managedFields 已省略。'
                      : '只读预览 · managedFields 已省略。'
                  }
                />
                {query.isError && <Alert type="error" showIcon title={errorMessage(query.error)} />}
                {query.isPending ? (
                  <div className="loading-panel">
                    <Spin />
                  </div>
                ) : (
                  query.data !== undefined && (
                    <Suspense
                      fallback={
                        <div className="loading-panel">
                          <Spin />
                        </div>
                      }
                    >
                      <YAMLViewer value={query.data} />
                    </Suspense>
                  )
                )}
              </>
            ),
          },
          ...(resource.group === '' && resource.resource === 'pods'
            ? [
                {
                  key: 'operations',
                  label: '日志与终端',
                  children: (
                    <Suspense fallback={<Spin />}>
                      <PodOperations
                        key={JSON.stringify(resource)}
                        target={resource}
                        expectedUID={expectedUID}
                      />
                    </Suspense>
                  ),
                },
              ]
            : []),
          {
            key: 'relations',
            label: '关联资源与事件',
            children: (
              <RelationsPanel
                key={JSON.stringify(resource)}
                target={resource}
                contextName={contextName}
                expectedUID={expectedUID}
              />
            ),
          },
        ]}
      />
    </Drawer>
  );
}
