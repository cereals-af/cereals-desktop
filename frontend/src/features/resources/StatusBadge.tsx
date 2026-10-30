const healthy = new Set(['Running', 'Ready', 'Active', 'Bound']);
const completed = new Set(['Completed', 'Complete', 'Succeeded']);
const pending = new Set([
  'Processing',
  'Pending',
  'Progressing',
  'Terminating',
  'ContainerCreating',
]);

export function StatusBadge({ status }: { status: string }) {
  const tone = healthy.has(status)
    ? 'healthy'
    : completed.has(status)
      ? 'completed'
      : status === 'Stopped'
        ? 'stopped'
        : pending.has(status)
          ? 'pending'
          : status === '—'
            ? 'neutral'
            : 'unhealthy';
  return (
    <span className={`status-badge ${tone}`}>
      <span />
      {status || '未知'}
    </span>
  );
}
