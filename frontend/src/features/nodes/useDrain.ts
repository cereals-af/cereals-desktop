import { useCallback, useEffect, useRef, useState } from 'react';
import { useAPI } from '../../api/context';
import type { DrainOptions, DrainPlan, DrainProgress, NodeSnapshot, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';

type Phase =
  | { kind: 'editing' }
  | { kind: 'previewing' }
  | { kind: 'review'; plan: DrainPlan }
  | { kind: 'starting'; plan: DrainPlan }
  | { kind: 'running'; id: string; progress?: DrainProgress };
export function useDrain(target: Ref, snapshot: NodeSnapshot, onSuccess: () => void) {
  const api = useAPI();
  const [phase, setPhase] = useState<Phase>({ kind: 'editing' });
  const [options, setOptions] = useState<DrainOptions>({
    ignoreDaemonSets: false,
    allowUnmanaged: false,
    allowLocalData: false,
    timeoutSeconds: 600,
  });
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);
  const lifetime = useRef(new AbortController());
  const token = useRef('');
  const task = useRef('');
  const notify = useRef(onSuccess);
  useEffect(() => {
    notify.current = onSuccess;
  }, [onSuccess]);
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    return () => {
      controller.abort();
      if (token.current) void api.discardDrain(token.current).catch(() => undefined);
      if (task.current) void api.closeDrain(task.current).catch(() => undefined);
    };
  }, [api]);
  const preview = async () => {
    setError('');
    setPhase({ kind: 'previewing' });
    const signal = lifetime.current.signal;
    try {
      const fresh = await api.node(target, signal);
      if (fresh.baseline.uid !== snapshot.baseline.uid)
        throw new Error('节点已被重新创建，请关闭并刷新节点详情');
      const plan = await api.previewDrain({ target, baseline: fresh.baseline, options }, signal);
      if (signal.aborted) {
        if (plan.token) await api.discardDrain(plan.token);
        return;
      }
      token.current = plan.token;
      setPhase({ kind: 'review', plan });
    } catch (failure) {
      if (!signal.aborted) {
        setError(errorMessage(failure));
        setPhase({ kind: 'editing' });
      }
    }
  };
  const back = () => {
    if (token.current) void api.discardDrain(token.current).catch(() => undefined);
    token.current = '';
    setPhase({ kind: 'editing' });
    setError('');
  };
  const start = async (confirmation: string) => {
    if (phase.kind !== 'review') return;
    const plan = phase.plan;
    setPhase({ kind: 'starting', plan });
    setError('');
    try {
      const id = await api.startDrain(plan.token, confirmation);
      token.current = '';
      if (lifetime.current.signal.aborted) {
        await api.closeDrain(id);
        return;
      }
      task.current = id;
      setPhase({ kind: 'running', id });
    } catch (failure) {
      if (!lifetime.current.signal.aborted) {
        setError(errorMessage(failure));
        setPhase({ kind: 'review', plan });
      }
    }
  };
  const id = phase.kind === 'running' ? phase.id : '';
  const poll = useCallback(
    async (signal: AbortSignal) => {
      if (!id) return false;
      try {
        const progress = await api.pollDrain(id, signal);
        if (signal.aborted) return false;
        setError('');
        setPhase({ kind: 'running', id, progress });
        if (progress.done) notify.current();
        return !progress.done;
      } catch (failure) {
        if (!signal.aborted) setError(errorMessage(failure));
        return false;
      }
    },
    [api, id],
  );
  useEffect(() => {
    if (!id) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const run = async () => {
      if (await poll(controller.signal)) timer = setTimeout(() => void run(), 1500);
    };
    void run();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [id, poll, revision]);
  const cancel = async () => {
    if (!id) return;
    try {
      await api.cancelDrain(id);
      await poll(lifetime.current.signal);
    } catch (failure) {
      setError(errorMessage(failure));
    }
  };
  return {
    phase,
    options,
    setOptions,
    error,
    preview,
    back,
    start,
    cancel,
    retry: () => setRevision((value) => value + 1),
  };
}
