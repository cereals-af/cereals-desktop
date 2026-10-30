import { describe, expect, it } from 'vitest';
import { property, setProperty, valueOf } from './values';

describe('lossless custom resource form values', () => {
  it('preserves unrendered fields and numeric text during a sibling edit', () => {
    const original = {
      ...valueOf('object'),
      properties: {
        sequence: valueOf('integer', '9007199254740993'),
        hidden: {
          ...valueOf('object'),
          properties: { exact: valueOf('integer', '9223372036854775807') },
        },
      },
    };
    const result = setProperty(original, 'label', valueOf('string', 'after'));
    expect(property(result, 'sequence')?.text).toBe('9007199254740993');
    expect(property(result, 'hidden')).toEqual(property(original, 'hidden'));
    expect(property(original, 'label')).toBeUndefined();
    expect(property(setProperty(result, 'label', undefined), 'label')).toBeUndefined();
    expect(property(setProperty(result, 'label', valueOf('null')), 'label')?.type).toBe('null');
  });
  it('treats property names as literal keys, including prototype and path-like keys', () => {
    const empty = valueOf('object');
    expect(property(empty, '__proto__')).toBeUndefined();
    const result = setProperty(
      setProperty(empty, '__proto__', valueOf('string', 'literal')),
      'a.b/c~d',
      valueOf('integer', '5'),
    );
    expect(property(result, '__proto__')?.text).toBe('literal');
    expect(property(result, 'a.b/c~d')?.text).toBe('5');
    expect(Object.getPrototypeOf(result.properties)).toBe(Object.prototype);
  });
});
