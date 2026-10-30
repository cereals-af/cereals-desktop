import { useEffect, useEffectEvent, useState } from 'react';
import { useAPI } from '../../api/context';
import type { Query, WatchUpdate } from '../../api/types';
import { errorMessage } from '../../lib/errors';

export function useResourceWatch(
  query: Query,
  version: string | undefined,
  enabled: boolean,
  onChange: () => void,
) {
  const api = useAPI();
  const [status, setStatus] = useState<WatchUpdate>({
    revision: 0,
    changes: 0,
    state: 'connecting',
    message: '',
  });
  const changed = useEffectEvent(onChange);
  // Keep the original list RV for the lifetime of this view. The server advances
  // its own cursor; refreshing the list must not tear down a healthy stream.
  const initial = useEffectEvent(() => version);
  const ready = Boolean(version);
  const { clusterID, group, resource, namespace, labelSelector } = query;
  const apiVersion = query.version;
  useEffect(() => {
    if (!enabled || !ready) return;
    const controller = new AbortController();
    let subscription = '';
    let timer: ReturnType<typeof setTimeout> | undefined;
    const run = async () => {
      try {
        subscription = await api.subscribe(
          {
            clusterID,
            group,
            version: apiVersion,
            resource,
            namespace,
            labelSelector,
            continue: '',
          },
          initial() ?? '',
        );
        let revision = 0;
        let changes = 0;
        while (!controller.signal.aborted) {
          const update = await api.poll(subscription, revision, controller.signal);
          if (controller.signal.aborted) break;
          setStatus(update);
          revision = update.revision;
          if (update.changes !== changes) {
            changes = update.changes;
            // Throttle bursts while preserving a final refresh under steady load.
            timer ??= setTimeout(() => {
              timer = undefined;
              changed();
            }, 500);
          }
          if (update.state === 'error') break;
        }
      } catch (failure) {
        if (!controller.signal.aborted)
          setStatus({ revision: 0, changes: 0, state: 'error', message: errorMessage(failure) });
      } finally {
        // Server leases bound the lifetime even if this cleanup cannot reach Go.
        if (subscription) void api.unsubscribe(subscription).catch(() => undefined);
      }
    };
    void run();
    return () => {
      controller.abort();
      if (timer) clearTimeout(timer);
    };
  }, [api, enabled, ready, clusterID, group, apiVersion, resource, namespace, labelSelector]);
  return enabled ? status : { ...status, state: 'paused', message: '' };
}
