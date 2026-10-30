import { useEffect, useRef, useState } from 'react';
import { useAPI } from '../../api/context';
import type { SessionRequest, SessionUpdate, TerminalSize } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { createTerminal, decodeBytes, encodeBytes, type TerminalView } from './terminal';

const initial: SessionUpdate = {
  cursor: 0,
  data: '',
  state: 'connecting',
  message: '',
  localPort: 0,
  exitCode: -1,
  done: false,
};
const logLimit = 512 * 1024;

export function usePodSession(request: SessionRequest) {
  const api = useAPI();
  const host = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState(initial);
  const [logs, setLogs] = useState('');
  const [truncated, setTruncated] = useState(false);
  const [id, setID] = useState('');
  const [controlError, setControlError] = useState('');
  useEffect(() => {
    const abort = new AbortController();
    let sessionID = '';
    let heartbeat: ReturnType<typeof setInterval> | undefined;
    let terminal: TerminalView | undefined;
    let ended = false;
    let disposed = false;
    let size: TerminalSize = { columns: request.columns, rows: request.rows };
    let inputQueue = new Uint8Array(0);
    let sending = false;
    const fail = (error: unknown) => {
      if (abort.signal.aborted || ended) return;
      setStatus((current) => ({
        ...current,
        state: 'error',
        message: errorMessage(error),
        done: true,
      }));
      abort.abort();
      terminal?.dispose();
      if (sessionID) void api.stopSession(sessionID).catch(() => undefined);
    };
    const drain = async () => {
      if (sending || !sessionID) return;
      sending = true;
      try {
        while (inputQueue.length && !abort.signal.aborted && !ended) {
          const bytes = inputQueue.slice(0, 4096);
          inputQueue = inputQueue.slice(bytes.length);
          await api.sessionInput(sessionID, encodeBytes(bytes));
        }
      } catch (error) {
        fail(error);
      } finally {
        sending = false;
      }
    };
    const input = (data: string) => {
      if (!sessionID || ended || abort.signal.aborted) return;
      const bytes = new TextEncoder().encode(data);
      if (inputQueue.length + bytes.length > 64 * 1024) {
        fail(new Error('终端输入过快，会话已停止；请重新连接后减少粘贴内容'));
        return;
      }
      const next = new Uint8Array(inputQueue.length + bytes.length);
      next.set(inputQueue);
      next.set(bytes, inputQueue.length);
      inputQueue = next;
      void drain();
    };
    const resize = (next: TerminalSize) => {
      size = next;
      if (sessionID && !ended && !abort.signal.aborted)
        void api.resizeSession(sessionID, size).catch(fail);
    };
    const run = async () => {
      const decoder = new TextDecoder();
      try {
        if (request.kind === 'exec' && host.current) {
          terminal = await createTerminal(host.current, input, resize);
          if (abort.signal.aborted) {
            terminal.dispose();
            return;
          }
        }
        // Keep the returned ID even if unmounted during Start, so finally can
        // release it. The Go validation call has its own bounded deadline.
        sessionID = await api.startSession({ ...request, ...size });
        if (abort.signal.aborted) return;
        setID(sessionID);
        heartbeat = setInterval(() => {
          void api.touchSession(sessionID).catch((error: unknown) => {
            if (!disposed) setControlError(errorMessage(error));
          });
        }, 15000);
        let cursor = 0;
        while (!abort.signal.aborted) {
          const update = await api.pollSession(sessionID, cursor, abort.signal);
          if (abort.signal.aborted) break;
          const bytes = decodeBytes(update.data);
          if (terminal) await terminal.write(bytes);
          else if (request.kind === 'logs') {
            const text = decoder.decode(bytes, { stream: !update.done });
            setLogs((current) => (current + text).slice(-logLimit));
            if (update.cursor > logLimit) setTruncated(true);
          }
          if (abort.signal.aborted) break;
          setStatus({ ...update, data: '' });
          cursor = update.cursor;
          if (update.done) break;
        }
      } catch (error) {
        fail(error);
      } finally {
        ended = true;
        // Completed sessions retain their archive until the view closes.
        if (disposed && sessionID) void api.closeSession(sessionID).catch(() => undefined);
      }
    };
    void run();
    return () => {
      disposed = true;
      abort.abort();
      clearInterval(heartbeat);
      terminal?.dispose();
      if (sessionID) void api.closeSession(sessionID).catch(() => undefined);
    };
  }, [api, request]);
  const stop = () => {
    if (id)
      void api.stopSession(id).catch((error: unknown) => setControlError(errorMessage(error)));
  };
  return { host, status, logs, truncated, stop, controlError, ready: Boolean(id), id };
}
