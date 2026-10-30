import { Tooltip } from 'antd';
import type { UsageSample } from '../../api/types';
import { cpuUsage, memoryUsage } from './format';

export function UsageCell({ sample, reason }: { sample?: UsageSample; reason: string }) {
  const usage = sample?.state === 'available' ? sample.usage : undefined;
  const title = usage
    ? `CPU ${cpuUsage(usage.cpuCores)} · 内存 ${memoryUsage(usage.memoryBytes)} · 采样于 ${new Date(sample?.timestamp ?? '').toLocaleTimeString()} · CPU 采样窗口 ${sample?.window} · 每 15 秒更新`
    : sample?.message || reason;
  return (
    <Tooltip title={title} trigger={['hover', 'focus']}>
      <span className="resource-usage" tabIndex={0} aria-label={title}>
        {usage ? (
          <>
            {cpuUsage(usage.cpuCores)}
            <span className="usage-divider">·</span>
            {memoryUsage(usage.memoryBytes)}
          </>
        ) : (
          '—'
        )}
      </span>
    </Tooltip>
  );
}
