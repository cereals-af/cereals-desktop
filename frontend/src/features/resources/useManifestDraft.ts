import { useEffect, useRef, useState } from 'react';
import { useAPI } from '../../api/context';
import type { Preview, Ref, Snapshot } from '../../api/types';
import { errorMessage } from '../../lib/errors';

type Phase =
  | { kind: 'editing' }
  | { kind: 'previewing' }
  | { kind: 'review'; preview: Preview }
  | { kind: 'committing'; preview: Preview }
  | { kind: 'done'; message: string };

export function useManifestDraft(
  target: Ref,
  snapshot: Snapshot,
  revealSecret: boolean,
  onSuccess: () => void,
) {
  const api = useAPI();
  const [draft, setDraft] = useState(snapshot.yaml);
  const [baseline, setBaseline] = useState(snapshot.baseline);
  const [mode, setMode] = useState(target.name ? 'patch' : 'create');
  const [phase, setPhase] = useState<Phase>({ kind: 'editing' });
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const lifetime = useRef(new AbortController());
  const token = phase.kind === 'review' || phase.kind === 'committing' ? phase.preview.token : '';

  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    return () => controller.abort();
  }, []);
  useEffect(
    () => () => {
      // The backend also expires tickets. Cleanup is best effort during window shutdown.
      if (token) void api.discard(token).catch(() => undefined);
    },
    [api, token],
  );

  const preview = async () => {
    const signal = lifetime.current.signal;
    setError('');
    setPhase({ kind: 'previewing' });
    try {
      const result = await api.preview(
        { target, mode, yaml: draft, baseline, revealSecret },
        signal,
      );
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
        setPhase({ kind: 'done', message: result.message });
        onSuccess();
      }
    } catch (failure) {
      if (!signal.aborted) {
        setError(`${errorMessage(failure)}。草稿已保留；网络中断时请先查询实际状态，再重新预检。`);
        setPhase({ kind: 'editing' });
      }
    }
  };
  const reload = async () => {
    const signal = lifetime.current.signal;
    setError('');
    setPhase({ kind: 'previewing' });
    try {
      const fresh = await api.snapshot(target, revealSecret, signal);
      if (!signal.aborted) {
        setBaseline(fresh.baseline);
        setNotice('已读取最新资源版本，保留了当前草稿。请在预检差异中逐项检查并发修改。');
      }
    } catch (failure) {
      if (!signal.aborted) setError(errorMessage(failure));
    } finally {
      if (!signal.aborted) setPhase({ kind: 'editing' });
    }
  };
  return {
    draft,
    setDraft,
    mode,
    setMode,
    phase,
    error,
    notice,
    preview,
    commit,
    reload,
    back: () => setPhase({ kind: 'editing' }),
  };
}
