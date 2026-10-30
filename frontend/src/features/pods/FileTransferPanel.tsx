import { useState } from 'react';
import { Alert, Button, Input, Progress, Tooltip } from 'antd';
import { FileUp, X } from 'lucide-react';
import type { SessionRequest } from '../../api/types';
import { useFileTransfer } from './useFileTransfer';

function fileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const unit = bytes >= 1024 ** 3 ? 3 : bytes >= 1024 ** 2 ? 2 : 1;
  return `${(bytes / 1024 ** unit).toLocaleString('zh-CN', { maximumFractionDigits: 2 })} ${['B', 'KiB', 'MiB', 'GiB'][unit]}`;
}
export function FileTransferPanel({
  sessionID,
  request,
  direction,
  onClose,
}: {
  sessionID: string;
  request: SessionRequest;
  direction: 'upload' | 'download';
  onClose: () => void;
}) {
  const [remote, setRemote] = useState(direction === 'upload' ? '/tmp' : '');
  const transfer = useFileTransfer(sessionID, direction);
  const progress = transfer.progress;
  const conflict = progress?.state === 'conflict';
  const percent = progress?.total
    ? Math.min(progress.done ? 100 : 99, Math.round((progress.bytes / progress.total) * 100))
    : 0;
  return (
    <section
      className="file-transfer-panel"
      aria-label={direction === 'upload' ? '上传文件面板' : '下载文件面板'}
    >
      <div className="file-transfer-heading">
        <strong>{direction === 'upload' ? '上传文件' : '下载文件'}</strong>
        <span className="muted">
          {request.target.namespace} / {request.target.name} / {request.container}
        </span>
        <Tooltip title={transfer.active ? '取消传输并关闭' : '关闭文件传输'}>
          <Button
            className="session-icon"
            aria-label="关闭文件传输"
            icon={<X size={16} />}
            onClick={onClose}
          />
        </Tooltip>
      </div>
      {!transfer.active && progress?.state !== 'completed' && (
        <>
          <div className="file-transfer-fields">
            {direction === 'upload' && (
              <div className="file-transfer-field">
                <span>本地文件</span>
                <div className="file-transfer-source">
                  <Button
                    aria-label="选择文件"
                    icon={<FileUp size={16} />}
                    loading={transfer.busy}
                    disabled={conflict}
                    onClick={() => void transfer.select()}
                  >
                    选择文件
                  </Button>
                  <span>
                    {transfer.selection
                      ? `${transfer.selection.name} · ${fileSize(transfer.selection.bytes)}`
                      : '选择不超过 2 GiB 的普通文件'}
                  </span>
                </div>
              </div>
            )}
            <label>
              {direction === 'upload' ? '容器目标目录' : '容器文件绝对路径'}
              <Input
                aria-label={direction === 'upload' ? '容器目标目录' : '容器文件绝对路径'}
                value={remote}
                disabled={conflict || transfer.busy}
                maxLength={4096}
                placeholder={direction === 'upload' ? '/tmp' : '/tmp/support.log'}
                onChange={(event) => setRemote(event.target.value)}
              />
            </label>
          </div>
          <p className="muted">
            普通文件 · 容器需要 /bin/sh 和 tar 等基础工具 · 独立传输，不占用交互终端
          </p>
        </>
      )}
      {transfer.error && <Alert showIcon type="error" title={transfer.error} />}
      {progress && (
        <>
          <div className="transfer-path">{progress.remotePath}</div>
          {transfer.active && (
            <>
              <Progress percent={percent} showInfo={Boolean(progress.total)} />
              <span className="muted">
                {fileSize(progress.bytes)}
                {progress.total ? ` / ${fileSize(progress.total)}` : ' · 正在读取文件信息'}
              </span>
            </>
          )}
          {progress.message && (
            <Alert
              showIcon
              type={
                progress.state === 'completed'
                  ? 'success'
                  : conflict
                    ? 'warning'
                    : progress.state === 'error'
                      ? 'error'
                      : 'info'
              }
              title={progress.message}
            />
          )}
        </>
      )}
      <div className="file-transfer-actions">
        {transfer.active ? (
          <Button onClick={transfer.cancel}>取消传输</Button>
        ) : (
          progress?.state !== 'completed' && (
            <Button
              type="primary"
              loading={transfer.busy}
              disabled={!remote.startsWith('/') || (direction === 'upload' && !transfer.selection)}
              onClick={() => void transfer.start(remote, conflict)}
            >
              {conflict
                ? '确认覆盖并上传'
                : direction === 'upload'
                  ? '开始上传'
                  : '选择保存位置并下载'}
            </Button>
          )
        )}
      </div>
    </section>
  );
}
