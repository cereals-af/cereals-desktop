import { useState } from 'react';
import { Alert, Button, Tooltip } from 'antd';
import { Download, Pause, Play } from 'lucide-react';
import { useAPI } from '../../api/context';
import { errorMessage } from '../../lib/errors';
import type { SessionRequest, SessionUpdate } from '../../api/types';
import { SessionToolbar } from './SessionToolbar';
import { LogOutput } from './LogOutput';

type ScrollMode =
  | { kind: 'auto' | 'static' }
  | { kind: 'paused'; cause: 'button' | 'scroll'; text: string; cursor: number };

export function LogSession({
  request,
  status,
  id,
  logs,
  truncated,
  stop,
}: {
  request: SessionRequest;
  status: SessionUpdate;
  id: string;
  logs: string;
  truncated: boolean;
  stop: () => void;
}) {
  const api = useAPI();
  const [mode, setMode] = useState<ScrollMode>({
    kind: request.follow && !request.previous ? 'auto' : 'static',
  });
  const [exporting, setExporting] = useState(false);
  const [notice, setNotice] = useState<{ kind: 'success' | 'error'; text: string } | null>(null);
  const paused = mode.kind === 'paused';
  const pause = (cause: 'button' | 'scroll') =>
    setMode({ kind: 'paused', cause, text: logs, cursor: status.cursor });
  const resume = () => setMode({ kind: 'auto' });
  const download = async () => {
    setExporting(true);
    setNotice(null);
    try {
      const saved = await api.exportLogs(id);
      if (!saved.cancelled)
        setNotice({
          kind: 'success',
          text: `已保存 ${saved.name}，共 ${saved.bytes.toLocaleString()} 字节`,
        });
    } catch (error) {
      setNotice({ kind: 'error', text: errorMessage(error) });
    } finally {
      setExporting(false);
    }
  };
  return (
    <>
      <SessionToolbar request={request} status={status} ready={Boolean(id)} stop={stop}>
        <span
          className={paused ? 'log-scroll-state paused' : 'log-scroll-state'}
          aria-live="polite"
        >
          {paused
            ? `已暂停滚动${status.cursor > mode.cursor ? ' · 有新日志' : ''}`
            : mode.kind === 'auto'
              ? '自动滚动'
              : '手动滚动'}
        </span>
        <Tooltip title={paused ? '恢复自动滚动' : '暂停自动滚动'}>
          <Button
            className="session-icon"
            aria-label={paused ? '恢复自动滚动' : '暂停自动滚动'}
            icon={paused ? <Play size={16} /> : <Pause size={16} />}
            disabled={!paused && (mode.kind === 'static' || status.done)}
            onClick={() => (paused ? resume() : pause('button'))}
          />
        </Tooltip>
        <Tooltip title="下载本次会话截至点击时已接收的全部日志">
          <Button
            className="session-icon"
            aria-label="下载本次会话日志"
            icon={<Download size={16} />}
            loading={exporting}
            disabled={!id || status.cursor === 0}
            onClick={() => void download()}
          />
        </Tooltip>
      </SessionToolbar>
      {notice && (
        <Alert
          type={notice.kind}
          title={notice.text}
          showIcon
          closable
          onClose={() => setNotice(null)}
        />
      )}
      {truncated && (
        <Alert type="info" title="视图仅保留最近约 512 Ki 字符；下载包含本次会话的完整归档" />
      )}
      <LogOutput
        logs={paused ? mode.text : logs}
        follow={mode.kind === 'auto'}
        cursor={paused ? mode.cursor : status.cursor}
        onScrollAway={() => {
          if (mode.kind === 'auto') pause('scroll');
        }}
        onScrollBottom={() => {
          if (mode.kind === 'paused' && mode.cause === 'scroll') resume();
        }}
      />
    </>
  );
}
