import { useState } from 'react';
import { Alert, Button, Input } from 'antd';
import { useAPI } from '../../api/context';
import type { Ref, VisualSnapshot } from '../../api/types';
import { property } from '../custom-resources/values';
import { FormSession } from '../resources/FormSession';
import { ResourceFields } from './ResourceFields';
import { suspended } from './models';
import type { VisualKind } from './resourceSchema';
interface Props {
  target: Ref;
  snapshot: VisualSnapshot;
  onSuccess: () => void;
  onDone: (target: Ref) => void;
  onBack: () => void;
}
export function RunJobEditor({ target, snapshot, onSuccess, onDone, onBack }: Props) {
  const api = useAPI();
  const [name] = useState(() => `${target.name.slice(0, 40)}-manual-${Date.now().toString(36)}`);
  return (
    <FormSession
      initial={name}
      baseline={snapshot.baseline}
      creating
      doneLabel="查看 Job"
      prepare={(name, baseline, signal) => api.previewRun({ target, baseline, name }, signal)}
      readBaseline={async (signal) => (await api.visualSnapshot(target, false, signal)).baseline}
      onSuccess={onSuccess}
      onDone={onDone}
      onBack={onBack}
    >
      {(name, setName) => (
        <>
          <Alert
            showIcon
            type="info"
            title="按当前 CronJob 模板创建一个独立 Job，提交时会再次检查模板版本。"
          />
          <div className="workload-fields">
            <label>
              新 Job 名称
              <Input
                aria-label="新 Job 名称"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </label>
          </div>
          <Button onClick={onBack}>返回详情</Button>
        </>
      )}
    </FormSession>
  );
}
export function VisualEditor({
  target,
  snapshot,
  onSuccess,
  onDone,
  onBack,
  kind,
  action,
}: Props & { kind: VisualKind; action?: 'suspend' }) {
  const api = useAPI();
  return (
    <FormSession
      initial={snapshot}
      baseline={snapshot.baseline}
      creating={!target.name}
      prepare={(draft, baseline, signal) =>
        api.previewVisual(
          {
            target: target.name
              ? target
              : {
                  ...target,
                  namespace: property(draft.value, 'namespace')?.text ?? target.namespace,
                },
            baseline,
            mode: target.name ? 'patch' : 'create',
            value: draft.value,
            entries: draft.entries,
            revealed: draft.revealed,
          },
          signal,
        )
      }
      readBaseline={async (signal) => (await api.visualSnapshot(target, false, signal)).baseline}
      onSuccess={onSuccess}
      onDone={onDone}
      onBack={onBack}
    >
      {(draft, setDraft) =>
        action === 'suspend' ? (
          <Alert
            showIcon
            type="warning"
            title={suspended(draft.value) ? `暂停 ${kind}` : `恢复 ${kind}`}
            description={
              kind === 'CronJob'
                ? '只改变后续调度；恢复后可能补运行错过的任务。'
                : '暂停 Job 会终止其活动 Pod；恢复后由控制器继续执行。'
            }
          />
        ) : (
          <ResourceFields
            snapshot={draft}
            kind={kind}
            creating={!target.name}
            clusterID={target.clusterID}
            onChange={setDraft}
          />
        )
      }
    </FormSession>
  );
}
