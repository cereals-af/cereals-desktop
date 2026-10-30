import { useLayoutEffect, useRef } from 'react';

export function LogOutput({
  logs,
  follow,
  cursor,
  onScrollAway,
  onScrollBottom,
}: {
  logs: string;
  follow: boolean;
  cursor: number;
  onScrollAway: () => void;
  onScrollBottom: () => void;
}) {
  const viewport = useRef<HTMLPreElement>(null);
  useLayoutEffect(() => {
    if (follow && viewport.current) viewport.current.scrollTop = viewport.current.scrollHeight;
  }, [logs, follow, cursor]);
  useLayoutEffect(() => {
    const element = viewport.current;
    if (!element || !follow) return;
    // Hidden session tabs have no height. Follow again when the tab or pane reappears.
    const observer = new ResizeObserver(() => {
      if (element.clientHeight > 0) element.scrollTop = element.scrollHeight;
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [follow]);
  return (
    <pre
      ref={viewport}
      className="pod-logs"
      aria-label="容器日志"
      onScroll={(event) => {
        const element = event.currentTarget;
        if (element.clientHeight === 0) return;
        if (element.scrollHeight - element.scrollTop - element.clientHeight <= 2) onScrollBottom();
        else onScrollAway();
      }}
    >
      {logs || '等待日志输出…'}
    </pre>
  );
}
