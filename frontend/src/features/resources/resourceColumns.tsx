import { Tag, Tooltip } from 'antd';
import type { TableColumnsType } from 'antd';
import { FileCode2 } from 'lucide-react';
import type { Item, ResourceType } from '../../api/types';
import { ageLabel } from './catalog';
import { StatusBadge } from './StatusBadge';

export function resourceColumns(
  type: ResourceType,
  openResource: (item: Item) => void,
): TableColumnsType<Item> {
  const columns: TableColumnsType<Item> = [
    {
      title: '名称',
      dataIndex: 'name',
      width: 310,
      render: (_, item) => (
        <div className="resource-name">
          <div className="resource-symbol">
            <FileCode2 size={18} />
          </div>
          <div>
            <button className="text-link" onClick={() => openResource(item)}>
              {item.name}
            </button>
            <span>{item.detail || type.kind}</span>
          </div>
        </div>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 165,
      render: (value: string) => <StatusBadge status={value} />,
    },
    {
      title: '命名空间',
      dataIndex: 'namespace',
      width: 170,
      render: (value: string) => <span className="namespace-label">{value || '集群作用域'}</span>,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 130,
      render: (value: string) => <Tooltip title={value}>{ageLabel(value)}</Tooltip>,
    },
  ];
  if (type.group === '' && type.resource === 'nodes') {
    columns.splice(
      2,
      1,
      {
        title: '调度',
        key: 'scheduling',
        width: 130,
        render: (_, item) => (
          <Tag color={item.node?.unschedulable ? 'orange' : 'green'}>
            {item.node?.unschedulable ? '已封锁' : '可调度'}
          </Tag>
        ),
      },
      {
        title: '角色 / IP',
        key: 'node',
        width: 180,
        render: (_, item) => (
          <div>
            {item.node?.roles || '—'}
            <div className="muted">{item.node?.internalIP || '—'}</div>
          </div>
        ),
      },
      {
        title: 'Kubelet',
        key: 'version',
        width: 130,
        render: (_, item) => item.node?.kubeletVersion || '—',
      },
    );
  }
  return columns;
}
