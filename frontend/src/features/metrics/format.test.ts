import { describe, expect, it } from 'vitest';
import { cpuUsage, memoryUsage } from './format';

describe('resource usage units', () => {
  it('keeps small nonzero quantities visible and switches at a core/GiB', () => {
    expect(cpuUsage(0.25)).toBe('250m');
    expect(cpuUsage(1.25)).toBe('1.25 核');
    expect(cpuUsage(0.0000001)).toBe('<0.01m');
    expect(cpuUsage(0)).toBe('0m');
    expect(memoryUsage(256 * 1024 ** 2)).toBe('256 MiB');
    expect(memoryUsage(1.8 * 1024 ** 3)).toBe('1.8 GiB');
    expect(memoryUsage(10)).toBe('<0.01 MiB');
  });
  it('does not present invalid values as zero', () => {
    expect(cpuUsage(NaN)).toBe('—');
    expect(memoryUsage(-1)).toBe('—');
  });
});
