import { useEffect, useRef, useState } from 'react';
import { useAPI } from '../../api/context';
import type { Baseline, Preview, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';

type Phase =
  | { kind: 'editing' }
  | { kind: 'previewing' }
  | { kind: 'review'; preview: Preview }
  | { kind: 'committing'; preview: Preview }
  | { kind: 'done'; target: Ref };
interface Options<T> {
  form: T;
  baseline: Baseline;
  prepare: (form: T, baseline: Baseline, signal: AbortSignal) => Promise<Preview>;
  readBaseline: (signal: AbortSignal) => Promise<Baseline>;
  onSuccess: () => void;
}

// Resource forms share one cancellable preview/commit lifecycle; each adapter owns its fields.
export function useFormDraft<T>(options: Options<T>) {
  const api = useAPI();
  const [form, setForm] = useState(options.form);
  const [baseline, setBaseline] = useState(options.baseline);
  const [notice, setNotice] = useState('');
  const [phase, setPhase] = useState<Phase>({ kind: 'editing' });
  const [error, setError] = useState('');
  const lifetime = useRef(new AbortController());
  const token = phase.kind === 'review' || phase.kind === 'committing' ? phase.preview.token : '';
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    return () => controller.abort();
  }, []);
  useEffect(
    () => () => {
      if (token) void api.discard(token).catch(() => undefined);
    },
    [api, token],
  );

  const preview = async () => {
    const signal = lifetime.current.signal;
    setError('');
    setPhase({ kind: 'previewing' });
    try {
      const result = await options.prepare(form, baseline, signal);
      if (signal.aborted) {
        void api.discard(result.token).catch(() => undefined);
        return;
      }
      setPhase({ kind: 'review', preview: result });
    } catch (failure) {
      if (!signal.aborted) {
        setError(errorMessage(failure));
        setPhase({ kind: 'editing' });
      }
    }
  };
  const commit = async () => {
    if (phase.kind !== 'review') return;
    const signal = lifetime.current.signal;
    setError('');
    setPhase({ kind: 'committing', preview: phase.preview });
    try {
      const result = await api.commit(phase.preview.token, signal);
      if (!signal.aborted) {
        setPhase({ kind: 'done', target: result.target });
        options.onSuccess();
      }
    } catch (failure) {
      if (!signal.aborted) {
        setError(`${errorMessage(failure)}。草稿已保留；请先检查资源当前状态。`);
        setPhase({ kind: 'editing' });
      }
    }
  };
  const reload = async () => {
    const signal = lifetime.current.signal;
    setError('');
    setPhase({ kind: 'previewing' });
    try {
      const fresh = await options.readBaseline(signal);
      if (!signal.aborted) {
        setBaseline(fresh);
        setNotice('已读取最新基线，当前表单草稿已保留。请逐项检查预检差异中的并发修改。');
      }
    } catch (failure) {
      if (!signal.aborted) setError(errorMessage(failure));
    } finally {
      if (!signal.aborted) setPhase({ kind: 'editing' });
    }
  };
  return {
    form,
    setForm,
    phase,
    error,
    notice,
    reload,
    preview,
    commit,
    back: () => setPhase({ kind: 'editing' }),
  };
}
