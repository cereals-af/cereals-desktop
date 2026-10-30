import { useQuery } from '@tanstack/react-query';
import { Button, Input, Select, Tooltip } from 'antd';
import { RefreshCw } from 'lucide-react';
import { useAPI } from '../../api/context';
import { errorMessage } from '../../lib/errors';
import { listNamespaces } from './namespaces';

interface Props {
  clusterID: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  allowAll?: boolean;
}

export function NamespaceSelect({
  clusterID,
  value,
  onChange,
  disabled = false,
  allowAll = true,
}: Props) {
  const api = useAPI();
  const query = useQuery({
    queryKey: ['cluster', clusterID, 'namespaces'],
    queryFn: ({ signal }) => listNamespaces(api, clusterID, signal),
    enabled: !disabled,
    staleTime: 30000,
    retry: false,
  });
  const names = new Set(query.data ?? []);
  if (value) names.add(value);
  return (
    <div className="namespace-select">
      <div className="namespace-control">
        {query.isError && !disabled ? (
          <Input
            aria-label="命名空间"
            value={value}
            allowClear={allowAll}
            placeholder={allowAll ? '输入名称；留空查询全部' : '输入命名空间'}
            onChange={(event) => onChange(event.target.value.trim())}
          />
        ) : (
          <Select
            aria-label="命名空间"
            showSearch
            optionFilterProp="label"
            disabled={disabled}
            loading={query.isFetching}
            value={disabled ? undefined : value || (allowAll ? '' : undefined)}
            placeholder={disabled ? '集群作用域' : '选择命名空间'}
            onChange={onChange}
            options={[
              ...(allowAll ? [{ value: '', label: '全部命名空间' }] : []),
              ...[...names].sort().map((name) => ({ value: name, label: name })),
            ]}
          />
        )}
        <Tooltip title="从集群重新读取命名空间">
          <Button
            aria-label="刷新命名空间"
            disabled={disabled}
            loading={query.isFetching}
            icon={<RefreshCw size={14} />}
            onClick={() => void query.refetch()}
          />
        </Tooltip>
      </div>
      {query.isError && !disabled && (
        <Tooltip title={errorMessage(query.error)}>
          <span className="namespace-hint">无法读取 Namespace 列表，请手动输入或重试</span>
        </Tooltip>
      )}
    </div>
  );
}
