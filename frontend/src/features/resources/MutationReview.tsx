import { lazy, Suspense } from 'react';
import { Alert, Descriptions, Spin, Tabs } from 'antd';
import type { Preview } from '../../api/types';

const Diff = lazy(() => import('./YAMLViewer').then((module) => ({ default: module.YAMLDiff })));

export function MutationReview({ preview }: { preview: Preview }) {
  return (
    <div className="mutation-review">
      <Descriptions
        size="small"
        column={2}
        items={[
          {
            key: 'name',
            label: '资源',
            children: `${preview.target.namespace || '集群作用域'} / ${preview.target.name}`,
          },
          {
            key: 'expiry',
            label: '预检有效期',
            children: new Date(preview.expiresAt).toLocaleTimeString(),
          },
        ]}
      />
      <Alert
        type="info"
        showIcon
        title="服务端预检已通过，尚未写入集群"
        description="左侧为修改前，右侧为修改后。正式提交仍会执行准入和并发校验；服务器生成字段及准入结果可能再次变化。"
      />
      <Suspense fallback={<Spin />}>
        <Tabs
          items={[
            {
              key: 'result',
              label: '最终变化',
              children: <Diff before={preview.before} after={preview.after} />,
            },
            ...(preview.mode === 'delete'
              ? []
              : [
                  {
                    key: 'draft',
                    label: '草稿变化',
                    children: <Diff before={preview.before} after={preview.desired} />,
                  },
                  {
                    key: 'admission',
                    label: '服务端处理差异',
                    children: <Diff before={preview.desired} after={preview.after} />,
                  },
                ]),
          ]}
        />
      </Suspense>
    </div>
  );
}
