import { useEffect, useRef, useState } from 'react';
import { useAPI } from '../../api/context';
import type { TransferProgress, TransferSelection } from '../../api/types';
import { errorMessage } from '../../lib/errors';

export function useFileTransfer(sessionID: string, direction: 'upload' | 'download') {
  const api = useAPI();
  const [selection, setSelection] = useState<TransferSelection | null>(null);
  const [jobID, setJobID] = useState('');
  const [progress, setProgress] = useState<TransferProgress | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const owned = useRef({ mounted: true, selection: '', job: '' });
  useEffect(() => {
    const owner = owned.current;
    owner.mounted = true;
    return () => {
      owner.mounted = false;
      // Best-effort close; backend ownership and leases also reclaim abandoned work.
      if (owner.job) void api.closeTransfer(owner.job).catch(() => undefined);
      if (owner.selection) void api.releaseUpload(owner.selection).catch(() => undefined);
    };
  }, [api]);
  useEffect(() => {
    if (!jobID) return;
    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      try {
        const value = await api.pollTransfer(jobID);
        if (disposed) return;
        setProgress(value);
        if (!value.done) timer = setTimeout(() => void poll(), 500);
      } catch (error) {
        if (!disposed) setError(errorMessage(error));
        void api.cancelTransfer(jobID).catch(() => undefined);
      }
    };
    void poll();
    return () => {
      disposed = true;
      clearTimeout(timer);
    };
  }, [api, jobID]);
  const select = async () => {
    setBusy(true);
    setError('');
    try {
      const next = await api.selectUpload(sessionID);
      if (!owned.current.mounted) {
        if (next.id) await api.releaseUpload(next.id);
        return;
      }
      if (!next.id) return;
      if (owned.current.selection) await api.releaseUpload(owned.current.selection);
      if (!owned.current.mounted) {
        await api.releaseUpload(next.id);
        return;
      }
      owned.current.selection = next.id;
      setSelection(next);
    } catch (error) {
      if (owned.current.mounted) setError(errorMessage(error));
    } finally {
      if (owned.current.mounted) setBusy(false);
    }
  };
  const start = async (remotePath: string, overwrite = false) => {
    setBusy(true);
    setError('');
    try {
      if (owned.current.job) {
        await api.closeTransfer(owned.current.job);
        owned.current.job = '';
      }
      if (!owned.current.mounted) return;
      setJobID('');
      setProgress(null);
      const id = await api.startTransfer({
        sessionID,
        direction,
        remotePath,
        overwrite,
        selectionID: selection?.id ?? '',
      });
      if (!owned.current.mounted) {
        if (id) await api.closeTransfer(id);
        return;
      }
      if (id) {
        owned.current.job = id;
        setProgress(null);
        setJobID(id);
      }
    } catch (error) {
      if (owned.current.mounted) setError(errorMessage(error));
    } finally {
      if (owned.current.mounted) setBusy(false);
    }
  };
  const cancel = () => {
    if (jobID)
      void api.cancelTransfer(jobID).catch((error: unknown) => {
        if (owned.current.mounted) setError(errorMessage(error));
      });
  };
  return {
    selection,
    progress,
    busy,
    error,
    select,
    start,
    cancel,
    active: Boolean(jobID && !progress?.done && !error),
  };
}
