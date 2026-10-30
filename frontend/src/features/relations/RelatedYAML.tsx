import { lazy, Suspense } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Drawer, Spin } from 'antd';
import { useAPI } from '../../api/context';
import type { RelatedNode } from '../../api/types';
import { errorMessage } from '../../lib/errors';

const YAMLViewer = lazy(() => import('../resources/YAMLViewer'));

export function RelatedYAML({
  node,
  contextName,
  onClose,
}: {
  node: RelatedNode;
  contextName: string;
  onClose: () => void;
}) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', node.target.clusterID, 'related-yaml', node.target, node.uid],
    queryFn: ({ signal }) => api.snapshot(node.target, false, signal),
    retry: false,
    gcTime: 0,
  });
  const replaced = query.data && query.data.baseline.uid !== node.uid;
  return (
    <Drawer
      open
      size={860}
      destroyOnHidden
      onClose={onClose}
      title={
        <div className="drawer-title">
          <span>
            {node.kind} · {node.target.name}
          </span>
          <small>
            {contextName} / {node.target.namespace} · 只读 YAML
          </small>
        </div>
      }
    >
      {query.isError && <Alert type="error" title={errorMessage(query.error)} />}
      {query.isPending && <Spin />}
      {replaced && <Alert type="warning" title="同名资源已被重新创建，请刷新关联列表后查看。" />}
      {!replaced && query.data && (
        <Suspense fallback={<Spin />}>
          <YAMLViewer value={query.data.yaml} />
        </Suspense>
      )}
    </Drawer>
  );
}
