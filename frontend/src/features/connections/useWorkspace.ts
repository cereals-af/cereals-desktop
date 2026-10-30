import { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useAPI } from '../../api/context';
import type { ConnectRequest } from '../../api/types';

export function useWorkspace() {
  const api = useAPI();
  const cache = useQueryClient();
  const [selectedID, setSelectedID] = useState('');
  const connectionController = useRef<AbortController | null>(null);
  useEffect(() => () => connectionController.current?.abort(), []);
  const workspace = useQuery({ queryKey: ['workspace'], queryFn: () => api.state(), retry: false });
  const profiles = workspace.data?.profiles ?? [];
  const selected =
    profiles.find((profile) => profile.id === selectedID) ??
    profiles.find((profile) => profile.id === workspace.data?.defaultClusterID) ??
    profiles[0];

  const importConfig = useMutation({
    mutationFn: (source: 'file' | 'default') =>
      source === 'file' ? api.importFile() : api.importDefault(),
    onSuccess: async (state) => {
      await cache.cancelQueries({ queryKey: ['cluster'] });
      cache.removeQueries({ queryKey: ['cluster'] });
      cache.setQueryData(['workspace'], state);
    },
  });
  const connect = useMutation({
    mutationFn: (request: ConnectRequest) => {
      connectionController.current?.abort();
      connectionController.current = new AbortController();
      setSelectedID(request.clusterID);
      return api.connect(request, connectionController.current.signal);
    },
    onSettled: async () => {
      await cache.invalidateQueries({ queryKey: ['workspace'] });
    },
    onSuccess: async (result) => {
      await cache.cancelQueries({ queryKey: ['cluster', result.clusterID] });
      cache.removeQueries({ queryKey: ['cluster', result.clusterID] });
    },
  });
  const setDefault = useMutation({
    mutationFn: (id: string) => api.setDefaultCluster(id),
    onSuccess: (state) => cache.setQueryData(['workspace'], state),
  });
  const disconnect = useMutation({
    mutationFn: (id: string) => api.disconnect(id),
    onSuccess: async (_, id) => {
      await cache.cancelQueries({ queryKey: ['cluster', id] });
      cache.removeQueries({ queryKey: ['cluster', id] });
      await cache.invalidateQueries({ queryKey: ['workspace'] });
    },
  });
  return {
    workspace,
    profiles,
    selected,
    setSelectedID,
    importConfig,
    setDefault,
    connect,
    disconnect,
    cancelConnection: () => connectionController.current?.abort(),
  };
}
