import { useState } from 'react';
import { Alert, Button, Tooltip } from 'antd';
import { Upload, Download } from 'lucide-react';
import type { SessionRequest } from '../../api/types';
import { usePodSession } from './usePodSession';
import { LogSession } from './LogSession';
import { SessionToolbar } from './SessionToolbar';
import { FileTransferPanel } from './FileTransferPanel';

export function SessionPane({ request }: { request: SessionRequest }) {
  const { host, status, logs, truncated, stop, controlError, ready, id } = usePodSession(request);
  const [transfer, setTransfer] = useState<'upload' | 'download' | null>(null);
  return (
    <section className="pod-session">
      {request.kind !== 'logs' && (
        <SessionToolbar request={request} status={status} ready={ready} stop={stop}>
          {request.kind === 'exec' && (
            <>
              <Tooltip title="上传文件">
                <Button
                  className="session-icon"
                  aria-label="上传文件"
                  icon={<Upload size={16} />}
                  disabled={!ready || Boolean(transfer)}
                  onClick={() => setTransfer('upload')}
                />
              </Tooltip>
              <Tooltip title="下载文件">
                <Button
                  className="session-icon"
                  aria-label="下载文件"
                  icon={<Download size={16} />}
                  disabled={!ready || Boolean(transfer)}
                  onClick={() => setTransfer('download')}
                />
              </Tooltip>
            </>
          )}
        </SessionToolbar>
      )}
      {status.message && (
        <Alert showIcon type={status.state === 'error' ? 'error' : 'info'} title={status.message} />
      )}
      {controlError && <Alert type="error" title={controlError} />}
      {request.kind === 'exec' && (
        <>
          {transfer && (
            <FileTransferPanel
              key={transfer}
              sessionID={id}
              request={request}
              direction={transfer}
              onClose={() => setTransfer(null)}
            />
          )}
          <div ref={host} className="pod-terminal" aria-label="Pod 交互终端" />
          <p className="muted">
            命令：{request.command?.join(' ')} · TTY · 关闭页签结束此会话
            {status.exitCode >= 0 ? ` · 退出码 ${status.exitCode}` : ''}
          </p>
        </>
      )}
      {request.kind === 'logs' && (
        <LogSession
          request={request}
          status={status}
          id={id}
          logs={logs}
          truncated={truncated}
          stop={stop}
        />
      )}
      {request.kind === 'forward' && (
        <div className="forward-summary">
          <span className="eyebrow">本地 TCP 转发</span>
          <h3>
            {status.localPort ? `127.0.0.1:${status.localPort}` : '等待分配本地端口'} →{' '}
            {request.target.name}:{request.remotePort}
          </h3>
          <p>仅在运行中可用。远端 Pod 退出或网络中断后需手动重新建立连接。</p>
          <p className="muted">每个转发最多 16 个本地连接；本地端口为 0 时自动分配可用端口。</p>
        </div>
      )}
    </section>
  );
}
