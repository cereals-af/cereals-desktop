export function cpuUsage(cores: number): string {
  if (!Number.isFinite(cores) || cores < 0) return '—';
  if (cores > 0 && cores < 0.00001) return '<0.01m';
  const value = cores < 1 ? cores * 1000 : cores;
  return `${value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })}${cores < 1 ? 'm' : ' 核'}`;
}

export function memoryUsage(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '—';
  if (bytes > 0 && bytes < 0.01 * 1024 ** 2) return '<0.01 MiB';
  const gib = bytes >= 1024 ** 3;
  const value = bytes / 1024 ** (gib ? 3 : 2);
  return `${value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} ${gib ? 'GiB' : 'MiB'}`;
}
