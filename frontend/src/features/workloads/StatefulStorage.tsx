import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Table } from 'antd';
import { useAPI } from '../../api/context';
import type { ControllerSnapshot, Profile, Ref, RelatedNode } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { ClaimTemplates } from './ClaimTemplates';
import { StorageWorkspace } from '../storage/StorageWorkspace';
import { RelatedYAML } from '../relations/RelatedYAML';

export function StatefulStorage({
  target,
  profile,
  snapshot,
  onSuccess,
}: {
  target: Ref;
  profile: Profile;
  snapshot: ControllerSnapshot;
  onSuccess: () => void;
}) {
  const api = useAPI();
  const [selected, setSelected] = useState<RelatedNode | null>(null);
  const [yaml, setYAML] = useState<RelatedNode | null>(null);
  const query = useQuery({
    queryKey: ['cluster', target.clusterID, 'sts-storage', target],
    queryFn: ({ signal }) => api.relations(target, signal),
    retry: false,
    gcTime: 0,
  });
  const section = query.data?.sections?.find((s) => s.key === 'claims');
  return (
    <>
      <Alert
        showIcon
        type="warning"
        title={`缩容：${snapshot.form.retainScaled} · 删除 StatefulSet：${snapshot.form.retainDeleted}`}
        description="Retain 保留 PVC；Delete 由控制器删除符合条件的 PVC，底层数据是否删除还取决于 PV 的回收策略。"
      />
      <h3>PVC 模板（创建后只读）</h3>
      <ClaimTemplates
        clusterID={target.clusterID}
        value={snapshot.form.claims ?? []}
        creating={false}
        onChange={() => undefined}
      />
      <div className="relation-toolbar">
        <h3>关联 PVC</h3>
        <Button loading={query.isFetching} onClick={() => void query.refetch()}>
          刷新 PVC
        </Button>
      </div>
      <p className="muted">{section?.note}</p>
      {(query.isError || section?.error) && (
        <Alert showIcon type="error" title={section?.error || errorMessage(query.error)} />
      )}
      {section?.truncated && (
        <Alert showIcon type="warning" title="PVC 查询不完整，仍可能存在其他 PVC" />
      )}
      <Table<RelatedNode>
        size="small"
        rowKey="uid"
        dataSource={section?.items ?? []}
        pagination={{ pageSize: 10 }}
        columns={[
          {
            title: '名称',
            render: (_, node) => (
              <Button type="link" className="resource-link" onClick={() => setSelected(node)}>
                {node.target.name}
              </Button>
            ),
          },
          { title: '状态', dataIndex: 'status' },
          { title: '关联依据', dataIndex: 'relation' },
          {
            title: '操作',
            render: (_, node) => (
              <Button size="small" onClick={() => setSelected(node)}>
                详情 / 扩容
              </Button>
            ),
          },
        ]}
      />
      {selected && (
        <StorageWorkspace
          target={selected.target}
          profile={profile}
          canPatch
          onClose={() => setSelected(null)}
          onSuccess={onSuccess}
          onYAML={() => setYAML(selected)}
        />
      )}
      {yaml && (
        <RelatedYAML node={yaml} contextName={profile.context} onClose={() => setYAML(null)} />
      )}
    </>
  );
}
