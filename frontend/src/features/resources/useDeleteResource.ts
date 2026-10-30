import { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useAPI } from '../../api/context';
import type { Preview, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';

type Phase =
  | { kind: 'options' }
  | { kind: 'busy' }
  | { kind: 'review'; preview: Preview }
  | { kind: 'done'; message: string };

export function useDeleteResource(target: Ref, onSuccess: () => void) {
  const api = useAPI();
  const [policy, setPolicy] = useState('Background');
  const [useGrace, setUseGrace] = useState(false);
  const [grace, setGrace] = useState(30);
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [phase, setPhase] = useState<Phase>({ kind: 'options' });
  const controller = useRef(new AbortController());
  const token = useRef('');
  const snapshot = useQuery({
    queryKey: ['cluster', target.clusterID, 'delete-baseline', target],
    queryFn: ({ signal }) => api.snapshot(target, false, signal),
    gcTime: 0,
    retry: false,
    staleTime: Infinity,
  });
  useEffect(() => {
    const lifetime = new AbortController();
    controller.current = lifetime;
    return () => {
      lifetime.abort();
      // The backend lease also expires if shutdown prevents this cleanup call.
      if (token.current) void api.discard(token.current).catch(() => undefined);
    };
  }, [api]);
  const preview = async () => {
    if (!snapshot.data) return;
    setError('');
    setPhase({ kind: 'busy' });
    try {
      const result = await api.previewDelete(
        {
          target,
          baseline: snapshot.data.baseline,
          propagation: policy,
          useGrace,
          graceSeconds: grace,
        },
        controller.current.signal,
      );
      token.current = result.token;
      if (!controller.current.signal.aborted) setPhase({ kind: 'review', preview: result });
      else void api.discard(result.token).catch(() => undefined);
    } catch (failure) {
      if (!controller.current.signal.aborted) {
        setError(errorMessage(failure));
        setPhase({ kind: 'options' });
      }
    }
  };
  const commit = async () => {
    if (phase.kind !== 'review' || name !== target.name) return;
    setPhase({ kind: 'busy' });
    setError('');
    try {
      const result = await api.commit(token.current, controller.current.signal);
      if (!controller.current.signal.aborted) {
        setPhase({ kind: 'done', message: result.message });
        onSuccess();
      }
    } catch (failure) {
      if (!controller.current.signal.aborted) {
        setError(`${errorMessage(failure)}。请先刷新资源确认实际状态，再重新发起删除。`);
        setPhase({ kind: 'options' });
      }
    }
  };
  const back = () => {
    if (phase.kind !== 'review') return;
    // Cleanup is best effort; the backend also expires unused tickets.
    void api.discard(phase.preview.token).catch(() => undefined);
    setPhase({ kind: 'options' });
  };
  return {
    policy,
    setPolicy,
    useGrace,
    setUseGrace,
    grace,
    setGrace,
    name,
    setName,
    error,
    phase,
    snapshot,
    preview,
    commit,
    back,
  };
}
