import type { DesktopAPI, TransferProgress, TransferRequest } from '../api/types';
import { fixtureSessionRequest } from './sessionFixtures';

const jobs = new Map<
  string,
  { request: TransferRequest; progress: TransferProgress; ticks: number }
>();
export const transferFixtures: Pick<
  DesktopAPI,
  | 'selectUpload'
  | 'releaseUpload'
  | 'startTransfer'
  | 'pollTransfer'
  | 'cancelTransfer'
  | 'closeTransfer'
> = {
  selectUpload: (id) => {
    fixtureSessionRequest(id);
    return Promise.resolve({ id: crypto.randomUUID(), name: 'support.txt', bytes: 4 * 1024 ** 2 });
  },
  releaseUpload: () => Promise.resolve(),
  startTransfer: (request) => {
    fixtureSessionRequest(request.sessionID);
    const id = crypto.randomUUID();
    jobs.set(id, {
      request,
      ticks: 0,
      progress: {
        id,
        state: 'running',
        name: request.direction === 'upload' ? 'support.txt' : 'support.log',
        remotePath: request.remotePath,
        bytes: 0,
        total: 4 * 1024 ** 2,
        message: '',
        done: false,
      },
    });
    return Promise.resolve(id);
  },
  pollTransfer: (id) => {
    const job = jobs.get(id);
    if (!job) return Promise.reject(new Error('文件传输已关闭'));
    const progress = job.progress;
    if (!progress.done) {
      job.ticks++;
      if (job.request.remotePath.includes('denied'))
        Object.assign(progress, {
          state: 'error',
          message: '权限不足：当前身份无权访问 pods/exec',
          done: true,
        });
      else if (
        job.request.direction === 'upload' &&
        !job.request.overwrite &&
        job.request.remotePath === '/etc'
      )
        Object.assign(progress, {
          state: 'conflict',
          message: '容器中已存在同名文件，请确认覆盖后重试',
          done: true,
        });
      else if (job.ticks >= 8)
        Object.assign(progress, {
          state: 'completed',
          bytes: progress.total,
          message: '文件传输完成',
          done: true,
        });
      else progress.bytes = Math.round((progress.total * job.ticks) / 8);
    }
    return Promise.resolve({ ...progress });
  },
  cancelTransfer: (id) => {
    const job = jobs.get(id);
    if (job) Object.assign(job.progress, { state: 'cancelled', message: '传输已取消', done: true });
    return Promise.resolve();
  },
  closeTransfer: (id) => {
    jobs.delete(id);
    return Promise.resolve();
  },
};
export function fixtureTransferCount() {
  return jobs.size;
}
