import { expect, test } from 'vitest';
import { nodeQuantity } from './quantities';
test('node capacity converts binary and decimal bytes without changing counts or CPU', () => {
  expect(nodeQuantity('memory', '33554432Ki')).toBe('32 GiB');
  expect(nodeQuantity('ephemeral-storage', '10737418240')).toBe('10 GiB');
  expect(nodeQuantity('memory', '16G')).toBe('14.9 GiB');
  expect(nodeQuantity('hugepages-2Mi', '512Mi')).toBe('512 MiB');
  expect(nodeQuantity('memory', '0')).toBe('0 GiB');
  expect(nodeQuantity('memory', 'invalid')).toBe('invalid');
  expect(nodeQuantity('cpu', '3900m')).toBe('3900m');
  expect(nodeQuantity('pods', '110')).toBe('110');
});
