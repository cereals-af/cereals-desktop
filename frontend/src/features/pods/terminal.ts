import type { TerminalSize } from '../../api/types';
import '@xterm/xterm/css/xterm.css';

export interface TerminalView {
  write(data: Uint8Array): Promise<void>;
  dispose(): void;
}

export async function createTerminal(
  host: HTMLElement,
  input: (data: string) => void,
  resize: (size: TerminalSize) => void,
): Promise<TerminalView> {
  const [{ Terminal }, { FitAddon }] = await Promise.all([
    import('@xterm/xterm'),
    import('@xterm/addon-fit'),
  ]);
  const style = getComputedStyle(host);
  const terminal = new Terminal({
    cursorBlink: true,
    scrollback: 3000,
    fontSize: 13,
    fontFamily: 'Menlo, Consolas, monospace',
    theme: {
      background: style.getPropertyValue('--terminal-bg').trim(),
      foreground: style.getPropertyValue('--terminal-fg').trim(),
      cursor: style.getPropertyValue('--color-brand').trim(),
    },
    allowProposedApi: false,
  });
  const fit = new FitAddon();
  terminal.loadAddon(fit);
  terminal.open(host);
  let disposed = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const fitSize = () => {
    if (host.clientWidth > 0 && host.clientHeight > 0) {
      fit.fit();
      resize({ columns: Math.min(500, terminal.cols), rows: Math.min(300, terminal.rows) });
    }
  };
  const observer = new ResizeObserver(() => {
    clearTimeout(timer);
    timer = setTimeout(fitSize, 80);
  });
  observer.observe(host);
  const subscription = terminal.onData(input);
  fitSize();
  terminal.focus();
  // Await xterm's parser before acknowledging output to Go. A hidden or slow
  // renderer therefore cannot accumulate an unbounded terminal write queue.
  const pending = new Set<() => void>();
  return {
    write: (bytes) =>
      new Promise<void>((resolve) => {
        if (disposed) {
          resolve();
          return;
        }
        pending.add(resolve);
        terminal.write(bytes, () => {
          pending.delete(resolve);
          resolve();
        });
      }),
    dispose: () => {
      if (disposed) return;
      disposed = true;
      clearTimeout(timer);
      observer.disconnect();
      subscription.dispose();
      terminal.dispose();
      for (const resolve of pending) resolve();
      pending.clear();
    },
  };
}

export function encodeBytes(data: Uint8Array): string {
  return btoa(Array.from(data, (byte) => String.fromCharCode(byte)).join(''));
}
export function decodeBytes(data: string): Uint8Array {
  return Uint8Array.from(atob(data), (character) => character.charCodeAt(0));
}
