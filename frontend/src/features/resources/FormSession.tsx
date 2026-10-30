import type { ReactNode } from 'react';
import { Alert, Button, ConfigProvider } from 'antd';
import type { Baseline, Preview, Ref } from '../../api/types';
import { MutationReview } from './MutationReview';
import { useFormDraft } from './useFormDraft';

interface Props<T> {
  initial: T;
  baseline: Baseline;
  creating: boolean;
  doneLabel?: string;
  prepare: (form: T, baseline: Baseline, signal: AbortSignal) => Promise<Preview>;
  readBaseline: (signal: AbortSignal) => Promise<Baseline>;
  onSuccess: () => void;
  onDone: (target: Ref) => void;
  onBack: () => void;
  children: (form: T, setForm: (value: T) => void) => ReactNode;
}

export function FormSession<T>(props: Props<T>) {
  const edit = useFormDraft({
    form: props.initial,
    baseline: props.baseline,
    prepare: props.prepare,
    readBaseline: props.readBaseline,
    onSuccess: props.onSuccess,
  });
  const busy = edit.phase.kind === 'previewing' || edit.phase.kind === 'committing';
  if (edit.phase.kind === 'done') {
    const target = edit.phase.target;
    return (
      <Alert
        showIcon
        type="success"
        title="请求已提交"
        description="API Server 已接受请求；请重新查看资源以确认实际结果。"
        action={
          <Button onClick={() => props.onDone(target)}>{props.doneLabel ?? '查看资源'}</Button>
        }
      />
    );
  }
  return (
    <div className="manifest-session">
      {edit.error && <Alert showIcon type="error" title={edit.error} />}
      {edit.notice && <Alert showIcon type="warning" title={edit.notice} />}
      {edit.phase.kind === 'review' || edit.phase.kind === 'committing' ? (
        <>
          <MutationReview preview={edit.phase.preview} />
          <div className="mutation-actions">
            <Button disabled={busy} onClick={edit.back}>
              返回修改
            </Button>
            <Button
              type="primary"
              aria-label="确认提交到集群"
              loading={busy}
              onClick={() => void edit.commit()}
            >
              确认提交到集群
            </Button>
          </div>
        </>
      ) : (
        <>
          <ConfigProvider componentDisabled={busy}>
            <fieldset disabled={busy} className="workload-fieldset">
              {props.children(edit.form, edit.setForm)}
            </fieldset>
          </ConfigProvider>
          <div className="mutation-actions">
            {!props.creating && (
              <>
                <Button disabled={busy} onClick={() => void edit.reload()}>
                  重新读取基线（保留草稿）
                </Button>
                <Button disabled={busy} onClick={props.onBack}>
                  放弃草稿，返回详情
                </Button>
              </>
            )}
            <Button type="primary" loading={busy} onClick={() => void edit.preview()}>
              服务端预检并查看差异
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
