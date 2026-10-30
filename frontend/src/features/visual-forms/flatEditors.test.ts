import { expect, test } from 'vitest';
import { property, setProperty, valueOf } from '../custom-resources/values';
import {
  affinityRules,
  affinityTypes,
  preferredAffinity,
  requiredAffinity,
  setAffinityRules,
} from './affinity';
import {
  changeEnvironmentSource,
  environmentSource,
  environmentSources,
  updateEnvironmentReference,
} from './environment';
import { containerGroups, podGroups } from './groupDefinitions';
import { tree } from './models';
import { container, podOptions } from './podSchema';

test('editing a Secret key retains optional settings, future fields and other environment data', () => {
  const entry = tree({
    name: 'TOKEN',
    futureEnv: true,
    valueFrom: {
      futureSource: 'keep',
      secretKeyRef: { name: 'credentials', key: 'old', optional: true, futureRef: 123 },
    },
  });
  const reference =
    property(property(entry, 'valueFrom') ?? valueOf('object'), 'secretKeyRef') ??
    valueOf('object');
  const changed = updateEnvironmentReference(
    entry,
    'secretKeyRef',
    setProperty(reference, 'key', valueOf('string', 'new')),
  );
  expect(changed).toEqual(
    tree({
      name: 'TOKEN',
      futureEnv: true,
      valueFrom: {
        futureSource: 'keep',
        secretKeyRef: { name: 'credentials', key: 'new', optional: true, futureRef: 123 },
      },
    }),
  );
  expect(changeEnvironmentSource(entry, 'secretKeyRef')).toBe(entry);
});

test.each(environmentSources)(
  'switching to $label replaces only the mutually exclusive environment source',
  ({ value: source }) => {
    const before = tree({
      name: 'SETTING',
      futureEnv: true,
      valueFrom: { unknownReference: { key: 'old' } },
    });
    expect(environmentSource(before)).toBe('unknown');
    const changed = changeEnvironmentSource(before, source);
    expect(environmentSource(changed)).toBe(source);
    expect(property(changed, 'name')?.text).toBe('SETTING');
    expect(property(changed, 'futureEnv')?.text).toBe('true');
    expect(
      source === 'value' ? property(changed, 'valueFrom') : property(changed, 'value'),
    ).toBeUndefined();
  },
);

for (const { value: type } of affinityTypes) {
  test.each([requiredAffinity, preferredAffinity])(
    `${type} %s retains other rules and unknown fields`,
    (mode) => {
      const other = tree([{ weight: 70, podAffinityTerm: { topologyKey: 'zone' } }]);
      const original = setProperty(
        tree({ futureAffinity: true, nodeAffinity: { futureNode: true } }),
        'podAntiAffinity',
        setProperty(valueOf('object'), preferredAffinity, other),
      );
      const rules = tree([
        { matchExpressions: [{ key: 'disk', operator: 'In', values: ['ssd'] }], futureRule: true },
      ]);
      const updated = setAffinityRules(original, type, mode, rules);
      expect(affinityRules(updated, type, mode)).toEqual(rules);
      expect(property(updated, 'futureAffinity')?.text).toBe('true');
      expect(
        property(property(updated, 'nodeAffinity') ?? valueOf('object'), 'futureNode')?.text,
      ).toBe('true');
      if (type !== 'podAntiAffinity' || mode !== preferredAffinity)
        expect(affinityRules(updated, 'podAntiAffinity', preferredAffinity)).toEqual(other);
      expect(
        affinityRules(setAffinityRules(updated, type, mode, undefined), type, mode),
      ).toBeUndefined();
      expect(affinityRules(original, type, mode)).toEqual(
        type === 'podAntiAffinity' && mode === preferredAffinity ? other : undefined,
      );
    },
  );
}

test('empty required node rules remain explicit until the user clears them', () => {
  const empty = tree([]);
  const updated = setAffinityRules(valueOf('object'), 'nodeAffinity', requiredAffinity, empty);
  expect(affinityRules(updated, 'nodeAffinity', requiredAffinity)).toEqual(empty);
  expect(setAffinityRules(updated, 'nodeAffinity', requiredAffinity, undefined)).toEqual(
    valueOf('object'),
  );
});

test('grouped editors keep every existing container and Pod option reachable', () => {
  const containerKeys = [...['name', 'image'], ...containerGroups.flatMap((group) => group.fields)];
  const podKeys = podGroups.flatMap((group) => group.fields);
  expect(container.fields?.every((field) => containerKeys.includes(field.key))).toBe(true);
  expect(podOptions.every((field) => podKeys.includes(field.key))).toBe(true);
  expect(new Set(containerKeys).size).toBe(containerKeys.length);
  expect(new Set(podKeys).size).toBe(podKeys.length);
});
