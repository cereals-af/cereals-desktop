import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Drawer, Space, Spin, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { Ref, RelatedNode } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { RelatedYAML } from './RelatedYAML';
import { RelationTables } from './RelationTables';
import { PodConsoles } from '../pods/PodConsoles';
import { usePodConsoles } from '../pods/usePodConsoles';

export function RelationsPanel({
  target,
  contextName,
  expectedUID,
}: {
  target: Ref;
  contextName: string;
  expectedUID?: string;
}) {
  const api = useAPI();
  const consoles = usePodConsoles();
  const [consoleOpen, setConsoleOpen] = useState(false);
  const [trail, setTrail] = useState<{ target: Ref; uid?: string }[]>([
    { target, uid: expectedUID },
  ]);
  const [yaml, setYAML] = useState<RelatedNode | null>(null);
  const current = trail.at(-1) ?? { target };
  const query = useQuery({
    queryKey: ['cluster', current.target.clusterID, 'relations', current.target, current.uid],
    queryFn: ({ signal }) => api.relations(current.target, signal),
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const replaced = Boolean(current.uid && query.data && current.uid !== query.data.root.uid);
  return (
    <div className="relations-panel">
      <div className="relation-toolbar">
        <Space wrap>
          {trail.length > 1 && (
            <Button onClick={() => setTrail((history) => history.slice(0, -1))}>
              返回上级关联
            </Button>
          )}
          <Tag>
            {contextName} / {current.target.namespace || '集群作用域'}
          </Tag>
          <strong>{current.target.name}</strong>
        </Space>
        <Button loading={query.isFetching} onClick={() => void query.refetch()}>
          刷新关联
        </Button>
      </div>
      {query.isError && (
        <Alert
          type="error"
          showIcon
          title={errorMessage(query.error)}
          description="无法完成关联查询，上次结果可能已过期。"
        />
      )}
      {query.isPending && <Spin />}
      {replaced && (
        <Alert
          type="warning"
          showIcon
          title="此名称对应的 UID 已变化，原资源可能已删除。请返回上级刷新后重新选择。"
        />
      )}
      {query.data && !replaced && (
        <>
          <p className="muted relation-note">
            查询于 {new Date(query.data.checkedAt).toLocaleTimeString()} · 按需读取，非事务快照 ·
            点击资源名称继续查看下级资源及事件
          </p>
          {query.data.sections?.map((section) => (
            <RelationTables
              key={section.key}
              section={section}
              onYAML={setYAML}
              onPod={(node) => {
                consoles.open(node);
                setConsoleOpen(true);
              }}
              onInspect={(node) =>
                setTrail((history) => [...history, { target: node.target, uid: node.uid }])
              }
            />
          ))}
        </>
      )}
      {consoles.entries.length > 0 && (
        <Drawer
          open={consoleOpen}
          size={1160}
          title={`Pod 操作 · ${contextName} · ${target.name}`}
          onClose={() => setConsoleOpen(false)}
        >
          <PodConsoles value={consoles} />
        </Drawer>
      )}
      {consoles.entries.length > 0 && (
        <Button onClick={() => setConsoleOpen(true)}>
          返回 Pod 会话（{consoles.entries.length}）
        </Button>
      )}
      {yaml && <RelatedYAML node={yaml} contextName={contextName} onClose={() => setYAML(null)} />}
    </div>
  );
}
