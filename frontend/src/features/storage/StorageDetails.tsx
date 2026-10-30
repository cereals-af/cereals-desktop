import { Alert, Descriptions, Table, Tag } from 'antd';
import type { Ref } from '../../api/types';
import { ClassCapability, ClassDetails } from './ClassDetails';
import type { StorageDocument } from './models';

export function StorageDetails({ document, target }: { document: StorageDocument; target: Ref }) {
  if (document.kind === 'class') return <ClassDetails value={document.value} />;
  if (document.kind === 'volume') {
    const v = document.value;
    return (
      <>
        <Descriptions
          bordered
          size="small"
          column={2}
          items={[
            { key: 'phase', label: '状态', children: v.phase || '未知' },
            { key: 'capacity', label: 'PV 声明容量', children: v.capacity || '—' },
            { key: 'modes', label: '访问模式', children: v.accessModes?.join(', ') },
            { key: 'mode', label: '卷模式', children: v.volumeMode },
            { key: 'class', label: 'StorageClass', children: v.className || '无' },
            { key: 'driver', label: '驱动 / 来源', children: v.driver },
            {
              key: 'claim',
              label: 'claimRef',
              children: v.claimName ? `${v.claimNamespace} / ${v.claimName}` : '未引用 PVC',
            },
            { key: 'uid', label: 'claimRef UID', children: v.claimUID || '—' },
            { key: 'policy', label: '回收策略', children: v.reclaimPolicy },
            { key: 'mount', label: '挂载选项', children: v.mountOptions?.join(', ') || '—' },
          ]}
        />
        {(v.reason || v.message) && (
          <Alert type="warning" showIcon title={v.reason || v.phase} description={v.message} />
        )}
        <Alert
          type="info"
          showIcon
          title="扩容请修改已绑定的 PVC 申请容量"
          description="PV 声明容量不等于后端磁盘或文件系统已完成扩容。绑定 UID 和使用者可在关联资源与事件中核对。"
        />
        {v.nodeAffinity && (
          <>
            <h3>节点亲和性</h3>
            <pre className="storage-json">{v.nodeAffinity}</pre>
          </>
        )}
        {v.className && <ClassCapability clusterID={target.clusterID} name={v.className} />}
      </>
    );
  }
  const c = document.value;
  return (
    <>
      {c.deleting && <Alert showIcon type="warning" title="PVC 正在删除，不能发起扩容" />}
      <Descriptions
        bordered
        size="small"
        column={2}
        items={[
          { key: 'phase', label: '状态', children: <Tag>{c.phase || '未知'}</Tag> },
          {
            key: 'class',
            label: 'StorageClass',
            children:
              c.className || (c.classMode === 'none' ? '显式无存储类' : '未指定（由集群默认处理）'),
          },
          { key: 'request', label: '申请容量', children: c.requested || '—' },
          { key: 'capacity', label: '已报告容量', children: c.capacity || '尚未报告' },
          { key: 'allocated', label: '已分配资源', children: c.allocated || '—' },
          { key: 'resize', label: '扩容状态', children: c.resizeStatus || '未报告' },
          { key: 'modes', label: '访问模式', children: c.accessModes?.join(', ') },
          { key: 'mode', label: '卷模式', children: c.volumeMode },
          { key: 'volume', label: '绑定 PV', children: c.volumeName || '尚未绑定' },
        ]}
      />
      <Alert
        showIcon
        type={c.phase === 'Bound' ? 'info' : 'warning'}
        title={c.phase === 'Bound' ? '申请与完成状态分别显示' : 'PVC 尚未处于 Bound 状态'}
        description="查看 Conditions、StorageClass 绑定模式及关联事件排查。提交扩容仅表示请求被接受，磁盘和文件系统处理进度由控制器报告。"
      />
      <h3>Conditions</h3>
      <Table
        dataSource={c.conditions ?? []}
        rowKey="type"
        size="small"
        pagination={false}
        locale={{ emptyText: '暂无条件；不代表文件系统或业务健康' }}
        columns={[
          { title: '类型', dataIndex: 'type' },
          { title: '状态', dataIndex: 'status' },
          { title: '原因', dataIndex: 'reason' },
          { title: '说明', dataIndex: 'message' },
        ]}
      />
      {c.className && <ClassCapability clusterID={target.clusterID} name={c.className} />}
    </>
  );
}
