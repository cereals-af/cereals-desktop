import type { ReactNode } from 'react';
import { Button, Tag } from 'antd';
import type { SessionRequest, SessionUpdate } from '../../api/types';

const states: Record<string, string> = {
  connecting: '正在连接',
  running: '运行中',
  completed: '已完成',
  stopped: '已停止',
  error: '连接失败 / 中断',
};
export function SessionToolbar({
  request,
  status,
  ready,
  stop,
  children,
}: {
  request: SessionRequest;
  status: SessionUpdate;
  ready: boolean;
  stop: () => void;
  children?: ReactNode;
}) {
  return (
    <div className="session-toolbar session-controls">
      <Tag
        color={status.state === 'running' ? 'green' : status.state === 'error' ? 'red' : 'default'}
      >
        {request.kind === 'logs' && status.state === 'running'
          ? '接收中'
          : (states[status.state] ?? status.state)}
      </Tag>
      <span className="muted">
        {request.container || 'Pod 网络'}
        {request.previous ? ' · 上一次运行' : ''}
      </span>
      <div className="session-actions">
        {children}
        <span className="session-action-divider" />
        <Button disabled={!ready || status.done} onClick={stop}>
          停止会话
        </Button>
      </div>
    </div>
  );
}
