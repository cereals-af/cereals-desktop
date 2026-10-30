import { Alert, Tabs } from 'antd';
import type { usePodConsoles } from './usePodConsoles';
import { PodOperations } from './PodOperations';

export function PodConsoles({ value }: { value: ReturnType<typeof usePodConsoles> }) {
  return (
    <>
      <p className="muted">
        选择另一个 Pod 只改变新会话目标。切换页签保留会话；关闭 Pod
        页签或父详情会结束对应会话，不自动连接重建后的 Pod。
      </p>
      {value.entries.length >= 8 && (
        <Alert
          type="info"
          title="最多保留 8 个 Pod 面板，请关闭不用的面板；集群会话总数仍受后端上限约束。"
        />
      )}
      <Tabs
        type="editable-card"
        hideAdd
        activeKey={value.active}
        onChange={value.setActive}
        onEdit={(key, action) => {
          if (action === 'remove' && typeof key === 'string') value.close(key);
        }}
        items={value.entries.map((pod) => ({
          key: pod.uid,
          label: `${pod.target.namespace} / ${pod.target.name}`,
          forceRender: true,
          children: <PodOperations target={pod.target} expectedUID={pod.uid} />,
        }))}
      />
    </>
  );
}
