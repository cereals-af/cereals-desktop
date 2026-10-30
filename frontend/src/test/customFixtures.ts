import { parseDocument } from 'yaml';
import type { CustomDefinition, CustomField, CustomValue, DesktopAPI, Ref } from '../api/types';
import { identity, property, valueOf } from '../features/custom-resources/values';
import { mutationFixtures } from './mutationFixtures';

const saved = new Map<string, CustomValue>();
const pending = new Map<string, { target: Ref; value: CustomValue }>();
const key = (target: Ref) =>
  `${target.clusterID}/${target.namespace}/${target.resource}/${target.name}`;
function tree(value: unknown): CustomValue {
  if (value === null) return valueOf('null');
  if (typeof value === 'bigint') return valueOf('integer', String(value));
  if (typeof value === 'number')
    return valueOf(Number.isInteger(value) ? 'integer' : 'number', String(value));
  if (typeof value === 'boolean' || typeof value === 'string')
    return valueOf(typeof value, String(value));
  if (Array.isArray(value)) return { ...valueOf('array'), items: value.map(tree) };
  if (typeof value === 'object')
    return {
      ...valueOf('object'),
      properties: Object.fromEntries(Object.entries(value).map(([key, item]) => [key, tree(item)])),
    };
  throw new Error('草稿类型无效');
}
function parse(yaml: string): CustomValue {
  const document = parseDocument(yaml, { intAsBigInt: true });
  if (document.errors.length) throw new Error('YAML 语法无效');
  const value: unknown = document.toJS();
  return tree(value);
}
function serialize(value: CustomValue): string {
  switch (value.type) {
    case 'object':
      return `{${Object.entries(value.properties ?? {})
        .flatMap(([name, item]) => (item ? [`${JSON.stringify(name)}:${serialize(item)}`] : []))
        .join(',')}}`;
    case 'array':
      return `[${value.items?.map(serialize).join(',') ?? ''}]`;
    case 'string':
      return JSON.stringify(value.text);
    case 'integer':
      if (
        !/^-?\d+$/.test(value.text) ||
        BigInt(value.text) > 9223372036854775807n ||
        BigInt(value.text) < -9223372036854775808n
      )
        throw new Error('整数超出 int64 范围');
      return value.text;
    case 'number':
      if (!Number.isFinite(Number(value.text))) throw new Error('数字无效');
      return value.text;
    case 'boolean':
      return value.text;
    case 'null':
      return 'null';
    default:
      throw new Error('未知类型');
  }
}
const field = (name: string, type: string, extra: Partial<CustomField> = {}): CustomField => ({
  name,
  type,
  description: '',
  required: false,
  nullable: false,
  enum: [],
  fields: [],
  items: null,
  reason: '',
  ...extra,
});
const definition: CustomDefinition = {
  name: 'widgets.demo.example.com',
  baseline: { uid: 'crd-widgets', resourceVersion: '80' },
  group: 'demo.example.com',
  resource: 'widgets',
  kind: 'Widget',
  namespaced: true,
  storedVersions: ['v1'],
  conditions: [
    {
      type: 'Established',
      status: 'True',
      reason: 'InitialNamesAccepted',
      message: '资源定义已建立',
    },
  ],
  versions: [
    {
      name: 'v1',
      served: true,
      storage: true,
      deprecated: false,
      warning: '',
      columns: [
        {
          name: 'Sequence',
          type: 'integer',
          description: '',
          jsonPath: '.spec.sequence',
          priority: 0,
        },
        { name: 'Label', type: 'string', description: '', jsonPath: '.spec.label', priority: 0 },
        { name: 'Advanced', type: 'string', description: '', jsonPath: '.spec.label', priority: 1 },
      ],
    },
    {
      name: 'v2',
      served: true,
      storage: false,
      deprecated: true,
      warning: '测试版本，建议使用 v1',
      columns: [
        { name: 'V2 Label', type: 'string', description: '', jsonPath: '.spec.label', priority: 0 },
      ],
    },
    { name: 'v3', served: false, storage: false, deprecated: false, warning: '', columns: [] },
  ],
};
function current(target: Ref): CustomValue {
  return structuredClone(
    saved.get(key(target)) ??
      parse(
        `{"apiVersion":"${target.group}/${target.version}","kind":"Widget","metadata":{"name":"${target.name}","namespace":"${target.namespace}"}${target.name ? ',"spec":{"sequence":9007199254740993,"label":"before","enabled":true,"items":[{"name":"first","opaque":9007199254740993}],"opaque":{"huge":9223372036854775807},"nullable":"initial"}' : ''}}`,
      ),
  );
}
export function commitCustom(token: string) {
  const item = pending.get(token);
  pending.delete(token);
  if (item) saved.set(key(item.target), item.value);
}
export function discardCustom(token: string) {
  pending.delete(token);
}
export const customFixtures: Pick<
  DesktopAPI,
  'customDefinition' | 'customList' | 'customSnapshot' | 'convertCustom' | 'previewCustom'
> = {
  customDefinition: (target) =>
    target.clusterID === 'test-staging'
      ? Promise.reject(new Error('权限不足：无法读取 CRD 定义'))
      : Promise.resolve(structuredClone(definition)),
  customList: (query) => {
    if (query.namespace === 'forbidden')
      return Promise.reject(new Error('权限不足：不能列出此命名空间的实例'));
    const columns = [
      { name: 'Name', type: 'string', description: '', jsonPath: '.metadata.name', priority: 0 },
      ...(definition.versions?.find((version) => version.name === query.version)?.columns ?? []),
    ];
    const target = {
      ...query,
      namespace: query.namespace || 'platform',
      name: query.continue ? 'second-widget' : 'sample-widget',
    };
    const value = current(target);
    const label = property(value, 'spec')?.properties?.label?.text ?? '';
    return Promise.resolve({
      columns,
      rows: [
        {
          uid: key(target),
          name: target.name,
          namespace: target.namespace,
          cells:
            query.version === 'v2'
              ? [target.name, label]
              : [target.name, '9007199254740993', label, label],
        },
      ],
      continue: query.continue ? '' : 'page-2',
      resourceVersion: '100',
    });
  },
  customSnapshot: (target) => {
    const value = current(target);
    return Promise.resolve({
      baseline: { uid: target.name ? key(target) : '', resourceVersion: target.name ? '100' : '' },
      definition: definition.baseline,
      draft: { value, yaml: serialize(value) },
      fields: [
        field('spec', 'object', {
          required: true,
          fields: [
            field('sequence', 'integer'),
            field('label', 'string', { required: true }),
            field('enabled', 'boolean'),
            field('mode', 'string', { enum: ['safe', 'fast'] }),
            field('nullable', 'string', { nullable: true }),
            field('items', 'array', {
              items: field('item', 'object', {
                fields: [field('name', 'string', { required: true })],
              }),
            }),
            field('opaque', 'object', { reason: '自由对象或映射，请使用 YAML' }),
          ],
        }),
      ],
    });
  },
  convertCustom: (value, yaml) => {
    const result = value ?? parse(yaml);
    return Promise.resolve({ value: result, yaml: serialize(result) });
  },
  previewCustom: async (request) => {
    const value = request.value ?? parse(request.yaml);
    const name = identity(value, 'name');
    if (name === 'schema-changed')
      throw new Error('CRD 定义已变化，请保留 YAML 草稿，刷新表单后重新检查');
    if (!name || !property(value, 'spec')?.properties?.label?.text)
      throw new Error('服务端校验失败：名称和 spec.label 必填');
    const target = { ...request.target, name, namespace: identity(value, 'namespace') };
    const preview = await mutationFixtures.preview({
      target,
      baseline: request.baseline,
      mode: request.mode === 'create' ? 'create' : 'jsonpatch',
      revealSecret: false,
      yaml: serialize(value),
    });
    pending.set(preview.token, { target, value: structuredClone(value) });
    return preview;
  },
};
