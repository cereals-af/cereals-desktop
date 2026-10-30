import { expect, test } from 'vitest';
import { convertSecretValue } from './secretEncoding';
test('UTF-8, newlines and empty values survive format changes', () => {
  const text = '配置:\n  password: 测试\n';
  expect(convertSecretValue(convertSecretValue(text, 'text', 'base64'), 'base64', 'text')).toBe(
    text,
  );
  expect(convertSecretValue('', 'text', 'base64')).toBe('');
  expect(() => convertSecretValue('AP8=', 'base64', 'text')).toThrow('UTF-8');
});
