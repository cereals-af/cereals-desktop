export type WorkloadAction = 'create' | 'update' | 'restart' | 'pause' | 'resume';
export const actionLabels: Record<WorkloadAction, string> = {
  create: '创建 Deployment',
  update: '可视化编辑',
  pause: '暂停发布',
  resume: '恢复发布',
  restart: '重启发布',
};
