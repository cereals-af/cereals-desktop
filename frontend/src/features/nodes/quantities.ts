const binary: Record<string, number> = {
  Ki: 1024,
  Mi: 1024 ** 2,
  Gi: 1024 ** 3,
  Ti: 1024 ** 4,
  Pi: 1024 ** 5,
  Ei: 1024 ** 6,
};
const decimal: Record<string, number> = {
  '': 1,
  n: 1e-9,
  u: 1e-6,
  m: 1e-3,
  k: 1e3,
  K: 1e3,
  M: 1e6,
  G: 1e9,
  T: 1e12,
  P: 1e15,
  E: 1e18,
};
// Display conversion only: never feed rounded quantities back into a resource update.
export function nodeQuantity(name: string, quantity: string): string {
  if (!['memory', 'ephemeral-storage', 'storage'].includes(name) && !name.startsWith('hugepages-'))
    return quantity;
  const match = /^([+-]?(?:\d+(?:\.\d*)?|\.\d+))([eE][+-]?\d+|[a-zA-Z]*)$/.exec(quantity);
  if (!match) return quantity;
  const suffix = match[2] ?? '';
  const multiplier =
    binary[suffix] ??
    decimal[suffix] ??
    (/^[eE][+-]?\d+$/.test(suffix) ? 10 ** Number(suffix.slice(1)) : NaN);
  const bytes = Number(match[1]) * multiplier;
  if (!Number.isFinite(bytes) || bytes < 0) return quantity;
  const unit = bytes > 0 && bytes < 1024 ** 3 ? 'MiB' : 'GiB';
  const value = bytes / (unit === 'MiB' ? 1024 ** 2 : 1024 ** 3);
  return `${value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} ${unit}`;
}
