import { useEffect, useState } from 'react';
import { useAPI } from '../../api/context';
import type { Ref, VisualSnapshot } from '../../api/types';
import { errorMessage } from '../../lib/errors';

// Sensitive values stay in this mounted workspace; they never enter the query cache.
export function useSnapshot(target: Ref, reveal: boolean, revision: number, enabled: boolean) {
  const api = useAPI();
  const [state, setState] = useState<{ data?: VisualSnapshot; error?: string; loading: boolean }>({
    loading: true,
  });
  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    void api
      .visualSnapshot(target, reveal, controller.signal)
      .then((data) => {
        if (!controller.signal.aborted) setState({ data, loading: false });
      })
      .catch((error) => {
        if (!controller.signal.aborted) setState({ error: errorMessage(error), loading: false });
      });
    return () => controller.abort();
  }, [api, target, reveal, revision, enabled]);
  return { ...state, clear: () => setState({ loading: true }) };
}
