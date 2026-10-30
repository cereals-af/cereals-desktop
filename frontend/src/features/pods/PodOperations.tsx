import { useId, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Checkbox, Input, InputNumber, Select, Space, Spin, Tabs, Tag } from 'antd';
import { useAPI } from '../../api/context';
import type { PodInfo, Ref, SessionRequest } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { SessionPane } from './SessionPane';

export function PodOperations({ target, expectedUID }: { target: Ref; expectedUID?: string }) {
  const api = useAPI();
  const pod = useQuery({
    queryKey: ['cluster', target.clusterID, 'pod-operations', target, expectedUID],
    queryFn: ({ signal }) => api.pod(target, signal),
    retry: false,
    gcTime: 0,
    staleTime: 0,
    refetchInterval: 5000,
    refetchOnWindowFocus: false,
  });
  if (pod.isPending)
    return (
      <div className="loading-panel">
        <Spin />
      </div>
    );
  if (!pod.data)
    return (
      <Alert
        type="error"
        showIcon
        title={errorMessage(pod.error)}
        action={<Button onClick={() => void pod.refetch()}>重试</Button>}
      />
    );
  if (expectedUID && pod.data.uid !== expectedUID)
    return (
      <Alert
        showIcon
        type="warning"
        title="Pod UID 已变化，原 Pod 已被替换。旧会话已结束，请返回列表刷新后重新选择。"
      />
    );
  return (
    <>
      <div className="session-toolbar">
        <span className="muted">容器状态按需刷新；Pod 重建后会结束旧会话。</span>
        <Button loading={pod.isFetching} onClick={() => void pod.refetch()}>
          刷新容器状态
        </Button>
      </div>
      {pod.isError && <Alert type="warning" title={errorMessage(pod.error)} />}
      <PodControls
        key={pod.data.uid}
        target={target}
        pod={pod.data}
        requireContainerChoice={Boolean(expectedUID)}
      />
    </>
  );
}

function PodControls({
  target,
  pod,
  requireContainerChoice,
}: {
  target: Ref;
  pod: PodInfo;
  requireContainerChoice: boolean;
}) {
  const selectorID = useId();
  const [container, setContainer] = useState(
    pod.containers?.length === 1 || !requireContainerChoice
      ? (pod.containers?.[0]?.name ?? '')
      : '',
  );
  const [follow, setFollow] = useState(true);
  const [previous, setPrevious] = useState(false);
  const [timestamps, setTimestamps] = useState(true);
  const [tail, setTail] = useState(500);
  const [since, setSince] = useState(0);
  const [shell, setShell] = useState('/bin/sh');
  const [local, setLocal] = useState(0);
  const [remote, setRemote] = useState(8080);
  const [sessions, setSessions] = useState<
    { key: string; request: SessionRequest; label: string }[]
  >([]);
  const [active, setActive] = useState('');
  const selected = pod.containers?.find((item) => item.name === container);
  const full = sessions.length >= 8;
  const start = (kind: string) => {
    if (full) return;
    const key = crypto.randomUUID();
    const request: SessionRequest = {
      target,
      uid: pod.uid,
      kind,
      container: kind === 'forward' ? '' : container,
      follow: follow && !previous,
      previous,
      timestamps,
      tailLines: tail,
      sinceSeconds: since,
      command: [shell],
      columns: 100,
      rows: 28,
      localPort: local,
      remotePort: remote,
    };
    const label =
      kind === 'logs'
        ? `日志 · ${target.name} / ${container}`
        : kind === 'exec'
          ? `终端 · ${target.name} / ${container}`
          : `转发 · ${target.name} / ${remote}`;
    setSessions((items) => [...items, { key, request, label }]);
    setActive(key);
  };
  const close = (key: string) => {
    setSessions((items) => items.filter((item) => item.key !== key));
    if (active === key) setActive(sessions.find((item) => item.key !== key)?.key ?? '');
  };
  return (
    <div className="pod-operations">
      <div className="pod-context">
        <Tag>{pod.phase}</Tag>
        <span>节点：{pod.node || '尚未调度'}</span>
        <span className="muted">Pod UID：{pod.uid}</span>
      </div>
      <p className="muted">
        会话绑定此 Pod。关闭详情或切换资源会停止全部会话；结束的会话不会自动重连。
      </p>
      <div className="pod-container-selector">
        <label htmlFor={selectorID}>容器</label>
        <Select
          id={selectorID}
          aria-label="Pod 容器"
          placeholder="选择明确的容器"
          value={container || undefined}
          onChange={setContainer}
          options={(pod.containers ?? []).map((item) => ({
            value: item.name,
            label: `${item.name} · ${item.kind} · ${item.state} · 重启 ${item.restarts} 次`,
          }))}
        />
      </div>
      {selected && selected.state !== 'running' && (
        <Alert
          showIcon
          type="info"
          title="所选容器未运行，终端不可连接；日志可独立尝试读取，也可选择上一次运行。"
        />
      )}
      <div className="pod-operation-grid">
        <section className="operation-card">
          <h3>容器日志</h3>
          <Space wrap>
            <Checkbox
              checked={follow && !previous}
              disabled={previous}
              onChange={(event) => setFollow(event.target.checked)}
            >
              持续接收新日志
            </Checkbox>
            <Checkbox checked={previous} onChange={(event) => setPrevious(event.target.checked)}>
              上一次运行
            </Checkbox>
            <Checkbox
              checked={timestamps}
              onChange={(event) => setTimestamps(event.target.checked)}
            >
              时间戳
            </Checkbox>
          </Space>
          <div className="operation-fields">
            <label>
              末尾行数
              <InputNumber
                aria-label="日志末尾行数"
                min={1}
                max={10000}
                precision={0}
                value={tail}
                onChange={(value) => setTail(value ?? 500)}
              />
            </label>
            <label>
              时间范围
              <Select
                aria-label="日志时间范围"
                value={since}
                onChange={setSince}
                options={[
                  { value: 0, label: '不限时间' },
                  { value: 300, label: '最近 5 分钟' },
                  { value: 3600, label: '最近 1 小时' },
                  { value: 86400, label: '最近 24 小时' },
                ]}
              />
            </label>
          </div>
          <Button disabled={!container || full} onClick={() => start('logs')}>
            打开日志
          </Button>
        </section>
        <section className="operation-card">
          <h3>交互终端</h3>
          <p className="muted">使用容器中的可执行程序启动 TTY。</p>
          <label>
            Shell 路径
            <Input
              aria-label="终端 Shell 路径"
              value={shell}
              maxLength={1024}
              onChange={(event) => setShell(event.target.value)}
            />
          </label>
          <Button
            type="primary"
            disabled={selected?.state !== 'running' || !shell.trim() || full}
            onClick={() => start('exec')}
          >
            连接终端
          </Button>
        </section>
        <section className="operation-card">
          <h3>端口转发</h3>
          <p className="muted">127.0.0.1 → Pod 网络中的 TCP 端口</p>
          <div className="operation-fields">
            <label>
              本地端口
              <InputNumber
                aria-label="本地端口"
                min={0}
                max={65535}
                precision={0}
                value={local}
                onChange={(value) => setLocal(value ?? 0)}
              />
            </label>
            <label>
              Pod 端口
              <InputNumber
                aria-label="Pod 端口"
                min={1}
                max={65535}
                precision={0}
                value={remote}
                onChange={(value) => setRemote(value ?? 8080)}
              />
            </label>
          </div>
          <Button disabled={pod.phase !== 'Running' || full} onClick={() => start('forward')}>
            开始转发
          </Button>
        </section>
      </div>
      {full && <Alert type="info" title="此面板最多保留 8 个会话，请先关闭不用的页签" />}
      {sessions.length > 0 ? (
        <Tabs
          type="editable-card"
          hideAdd
          activeKey={active}
          onChange={setActive}
          onEdit={(key, action) => {
            if (action === 'remove' && typeof key === 'string') close(key);
          }}
          items={sessions.map((item) => ({
            key: item.key,
            label: item.label,
            forceRender: true,
            children: <SessionPane request={item.request} />,
          }))}
        />
      ) : (
        <div className="session-empty">选择容器，打开日志或建立交互会话</div>
      )}
    </div>
  );
}
