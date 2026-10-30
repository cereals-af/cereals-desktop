import { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useAPI } from '../../api/context';
import type { UsageQuery, UsageTarget } from '../../api/types';
import { errorMessage } from '../../lib/errors';

export function usageKey(target: UsageTarget): string {
  return `${target.namespace}/${target.name}/${target.uid}`;
}

export function useResourceUsage(request: UsageQuery, enabled = true) {
  const api = useAPI();
  const host = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);
  useEffect(() => {
    const element = host.current;
    if (!element) return;
    const observer = new IntersectionObserver(([entry]) =>
      setVisible(Boolean(entry?.isIntersecting)),
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  const targets = [...(request.targets ?? [])].sort((a, b) =>
    usageKey(a).localeCompare(usageKey(b)),
  );
  const query = useQuery({
    queryKey: ['cluster', request.clusterID, 'usage', request.resource, request.namespace, targets],
    queryFn: ({ signal }) => api.resourceUsage({ ...request, targets }, signal),
    enabled: enabled && visible && targets.length > 0,
    refetchInterval: 15000,
    refetchIntervalInBackground: false,
    staleTime: 10000,
    gcTime: 0,
    retry: false,
  });
  // A failed refresh must not leave yesterday's value looking current.
  const samples = new Map(
    (!query.isError && query.data?.state === 'available' ? (query.data.items ?? []) : []).map(
      (sample) => [usageKey(sample.target), sample],
    ),
  );
  const reason = query.isError
    ? errorMessage(query.error)
    : query.data?.message || (query.isPending ? '正在读取资源用量' : '暂无采样');
  return { host, samples, reason, refresh: query.refetch };
}
