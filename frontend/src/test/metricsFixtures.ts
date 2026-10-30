import type { DesktopAPI } from '../api/types';

export const metricsFixtures: Pick<DesktopAPI, 'resourceUsage'> = {
  resourceUsage: (query) => {
    if (query.clusterID === 'test-staging')
      return Promise.resolve({
        state: 'forbidden',
        message: '当前身份无权读取资源用量',
        items: [],
      });
    const mode = sessionStorage.getItem('fixture-metrics');
    if (mode === 'unavailable')
      return Promise.resolve({
        state: 'unavailable',
        message: 'Metrics Server 未安装或暂不可用',
        items: [],
      });
    return Promise.resolve({
      state: 'available',
      message: '',
      items: (query.targets ?? []).map((target, index) => ({
        target,
        state:
          mode === 'missing' || target.name.includes('console')
            ? 'missing'
            : mode === 'stale'
              ? 'stale'
              : 'available',
        message:
          mode === 'stale' ? '采样已过期或集群时钟异常' : '暂无采样，容器可能尚未启动或已经结束',
        timestamp: new Date().toISOString(),
        window: '15s',
        usage: {
          cpuCores: query.resource === 'nodes' ? 2.35 : index === 0 ? 0.25 : 1.25,
          memoryBytes:
            query.resource === 'nodes'
              ? 8.6 * 1024 ** 3
              : index === 0
                ? 256 * 1024 ** 2
                : 1.8 * 1024 ** 3,
        },
      })),
    });
  },
};
