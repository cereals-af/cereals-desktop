export function convertSecretValue(value: string, from: string, to: string): string {
  if (from === to) return value;
  if (to === 'base64')
    return btoa(
      Array.from(new TextEncoder().encode(value), (byte) => String.fromCharCode(byte)).join(''),
    );
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(
      Uint8Array.from(atob(value), (char) => char.charCodeAt(0)),
    );
  } catch {
    throw new Error('当前内容不是有效 UTF-8 文本，请保留 Base64 格式。');
  }
}
