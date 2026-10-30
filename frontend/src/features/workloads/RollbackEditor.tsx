import { Alert, Descriptions } from 'antd';
import { useAPI } from '../../api/context';
import type { Baseline, DeploymentRevision, Ref } from '../../api/types';
import { FormSession } from '../resources/FormSession';

export function RollbackEditor({
  target,
  revision,
  baseline,
  onSuccess,
  onStatus,
  onBack,
  controller = false,
}: {
  controller?: boolean;
  target: Ref;
  revision: DeploymentRevision;
  baseline: Baseline;
  onSuccess: () => void;
  onStatus: (target: Ref) => void;
  onBack: () => void;
}) {
  const api = useAPI();
  return (
    <FormSession
      initial={revision}
      baseline={baseline}
      creating={false}
      prepare={(source, current, signal) =>
        controller
          ? api.previewControllerRollback(
              {
                target,
                baseline: current,
                name: source.name,
                source: source.baseline,
                revision: source.revision,
              },
              signal,
            )
          : api.previewRollback(
              {
                target,
                baseline: current,
                replicaSet: source.name,
                source: source.baseline,
                revision: source.revision,
              },
              signal,
            )
      }
      readBaseline={async (signal) =>
        (await (controller ? api.controller(target, signal) : api.deployment(target, signal)))
          .baseline
      }
      onSuccess={onSuccess}
      onDone={onStatus}
      onBack={onBack}
      doneLabel="查看发布状态"
    >
      {() => (
        <>
          <Alert
            type="warning"
            showIcon
            title={`准备回滚到修订 ${revision.revision}`}
            description="预检不会写入。请检查完整 Pod 模板差异，包括容器、环境变量、卷、调度及模板注解；正式提交仍会核对历史源对象。"
          />
          <Descriptions
            column={1}
            items={[
              {
                key: 'rs',
                label: controller ? '历史 ControllerRevision' : '历史 ReplicaSet',
                children: revision.name,
              },
              { key: 'uid', label: '历史对象 UID', children: revision.baseline.uid },
              { key: 'images', label: '历史镜像', children: revision.images?.join('；') },
            ]}
          />
          <p className="muted">
            重新读取基线会保留选定的历史对象。若历史对象变化或被删除，请返回发布历史刷新后重新选择。恢复发布后控制器可能更新修订号，需重新读取历史。
          </p>
        </>
      )}
    </FormSession>
  );
}
