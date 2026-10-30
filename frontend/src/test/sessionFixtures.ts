import type { DesktopAPI, Ref, SessionRequest, SessionUpdate } from '../api/types';

const replaced = new Set<string>();
export function fixturePodUID(target: Ref): string {
  return (
    [
      target.clusterID,
      target.group,
      target.version,
      target.resource,
      target.namespace,
      target.name,
    ].join('|') + (replaced.has(target.name) ? '-replaced' : '')
  );
}
export function replaceFixturePod(name: string) {
  replaced.add(name);
}
export function fixtureSessionCount() {
  return sessions.size;
}
interface FixtureSession {
  request: SessionRequest;
  bytes: Uint8Array;
  state: string;
  stopped: boolean;
  logSequence: number;
}
const sessions = new Map<string, FixtureSession>();
const append = (session: FixtureSession, text: string) => {
  const bytes = new TextEncoder().encode(text);
  const result = new Uint8Array(session.bytes.length + bytes.length);
  result.set(session.bytes);
  result.set(bytes, session.bytes.length);
  session.bytes = result;
};
export const sessionFixtures: Pick<
  DesktopAPI,
  | 'pod'
  | 'startSession'
  | 'pollSession'
  | 'sessionInput'
  | 'resizeSession'
  | 'stopSession'
  | 'closeSession'
  | 'touchSession'
  | 'exportLogs'
> = {
  touchSession: (id) =>
    sessions.has(id) ? Promise.resolve() : Promise.reject(new Error('会话已关闭')),
  exportLogs: (id) => {
    const session = sessions.get(id);
    if (!session) return Promise.reject(new Error('会话已关闭'));
    exported.push({ id, text: new TextDecoder().decode(session.bytes) });
    return Promise.resolve({
      name: `${session.request.container}.log`,
      bytes: session.bytes.length,
      cancelled: false,
    });
  },
  pod: (target) =>
    Promise.resolve({
      uid: fixturePodUID(target),
      phase: 'Running',
      node: 'node-pool-1',
      containers: [
        { name: 'app', kind: '普通', state: 'running', restarts: 2, ports: [8080] },
        { name: 'sidecar', kind: '普通', state: 'running', restarts: 0, ports: [] },
        { name: 'init-config', kind: 'Init', state: 'terminated', restarts: 0, ports: [] },
      ],
    }),
  startSession: (request) => {
    if (request.uid !== fixturePodUID(request.target))
      return Promise.reject(new Error('Pod 已被重建，请重新选择'));
    if (request.container === 'sidecar' && request.kind === 'exec')
      return Promise.reject(new Error('权限不足：当前身份无权连接容器终端'));
    const id = crypto.randomUUID();
    const session: FixtureSession = {
      request,
      bytes: new Uint8Array(),
      state: request.kind === 'logs' && !request.follow ? 'completed' : 'running',
      stopped: false,
      logSequence: 0,
    };
    sessions.set(id, session);
    if (request.kind === 'logs')
      append(
        session,
        `${request.timestamps ? '2026-10-27T09:00:00Z ' : ''}${request.container} ${request.previous ? 'previous: stopped before restart' : 'ready: 服务已启动'}\n`,
      );
    if (request.kind === 'logs' && request.tailLines >= 1000)
      append(session, Array.from({ length: 160 }, (_, i) => `history line ${i}\n`).join(''));
    if (request.kind === 'exec') append(session, '\u001b[32mConnected to app\u001b[0m\r\n$ ');
    return Promise.resolve(id);
  },
  pollSession: async (id, cursor, signal) => {
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        signal?.removeEventListener('abort', cancel);
        resolve();
      }, 120);
      const cancel = () => {
        clearTimeout(timer);
        reject(new DOMException('Aborted', 'AbortError'));
      };
      if (signal?.aborted) cancel();
      else signal?.addEventListener('abort', cancel, { once: true });
    });
    const session = sessions.get(id);
    if (!session) throw new Error('会话已关闭');
    if (
      session.request.kind === 'logs' &&
      session.request.follow &&
      session.request.tailLines >= 1000 &&
      !session.stopped &&
      session.logSequence < 180
    ) {
      session.logSequence++;
      append(session, `follow line ${session.logSequence}\n`);
    }
    const result: SessionUpdate = {
      cursor: session.bytes.length,
      data: btoa(
        Array.from(session.bytes.slice(cursor), (byte) => String.fromCharCode(byte)).join(''),
      ),
      state: session.stopped ? 'stopped' : session.state,
      message: session.stopped ? '会话已停止或集群连接已断开' : '',
      localPort: session.request.kind === 'forward' ? session.request.localPort || 49152 : 0,
      exitCode: session.state === 'completed' && session.request.kind === 'exec' ? 0 : -1,
      done: session.stopped || session.state === 'completed',
    };
    return result;
  },
  sessionInput: (id, data) => {
    const session = sessions.get(id);
    if (!session) return Promise.reject(new Error('会话已关闭'));
    const text = new TextDecoder().decode(
      Uint8Array.from(atob(data), (character) => character.charCodeAt(0)),
    );
    append(session, text.replaceAll('\r', '\r\n$ '));
    if (text.includes('exit')) {
      session.state = 'completed';
      append(session, '\r\nlogout\r\n');
    }
    return Promise.resolve();
  },
  resizeSession: () => Promise.resolve(),
  stopSession: (id) => {
    const session = sessions.get(id);
    if (session) session.stopped = true;
    return Promise.resolve();
  },
  closeSession: (id) => {
    sessions.delete(id);
    return Promise.resolve();
  },
};

const exported: { id: string; text: string }[] = [];
export function fixtureLogExports() {
  return exported;
}
export function fixtureSessionRequest(id: string): SessionRequest {
  const value = sessions.get(id);
  if (!value) throw new Error('会话已关闭');
  return value.request;
}

export function appendFixtureLogs(text: string) {
  for (const session of sessions.values())
    if (session.request.kind === 'logs') append(session, text);
}
