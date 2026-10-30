import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Space, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import YAMLViewer from './YAMLViewer';
import { parseEditorSchema } from './yamlLanguage';

export default function SchemaEditor({
  target,
  value,
  readOnly,
  onChange,
}: {
  target: Ref;
  value: string;
  readOnly: boolean;
  onChange: (value: string) => void;
}) {
  const api = useAPI();
  const query = useQuery({
    queryKey: [
      'cluster',
      target.clusterID,
      'schema',
      target.group,
      target.version,
      target.resource,
    ],
    queryFn: async ({ signal }) => {
      const document = await api.schema(target, signal);
      return { document, schema: parseEditorSchema(document.json) };
    },
    gcTime: 0,
    staleTime: 5 * 60 * 1000,
    retry: false,
    refetchOnWindowFocus: false,
  });
  return (
    <div className="schema-editor">
      <Space wrap className="schema-toolbar">
        <Tag color={query.isError ? 'warning' : query.data ? 'success' : 'default'}>
          {query.isError
            ? 'Schema 不可用'
            : query.data
              ? `${query.data.document.source} · ${query.data.document.kind}`
              : '正在读取 Schema'}
        </Tag>
        <span className="muted">字段补全 / 悬停说明 / 行内校验 · 提交以服务端预检为准</span>
        <Button size="small" loading={query.isFetching} onClick={() => void query.refetch()}>
          刷新 Schema
        </Button>
      </Space>
      {query.isError && (
        <Alert
          type="warning"
          showIcon
          title={errorMessage(query.error)}
          description="Schema 不可用不影响编辑和服务端预检；不会自动下载外部 Schema。"
        />
      )}
      <YAMLViewer
        value={value}
        readOnly={readOnly}
        onChange={onChange}
        schema={query.isError ? undefined : query.data?.schema}
      />
    </div>
  );
}
