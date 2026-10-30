import { bool, choice, list, map, number, object, port, strings, text, type Field } from './schema';
import { labelSelector, podTemplate } from './podSchema';

export type VisualKind = 'Job' | 'CronJob' | 'Ingress' | 'NetworkPolicy' | 'Secret';
export const metadataFields = [map('labels', '标签'), map('annotations', '注解')];
const jobFields = [
  number('parallelism', '并行数'),
  number('completions', '完成次数'),
  text('completionMode', '完成模式', ['NonIndexed', 'Indexed']),
  number('backoffLimit', '失败重试上限'),
  number('activeDeadlineSeconds', '最长运行秒数'),
  number('ttlSecondsAfterFinished', '完成后保留秒数'),
  bool('suspend', '暂停执行'),
  number('backoffLimitPerIndex', '每个索引重试上限'),
  number('maxFailedIndexes', '最多失败索引'),
  podTemplate,
];
const backend = (key: string, label: string) =>
  choice(key, label, [
    object('service', 'Service', [
      text('name', 'Service 名称'),
      choice('port', 'Service 端口', [number('number', '端口号'), text('name', '命名端口')]),
    ]),
    object('resource', '资源引用', [
      text('apiGroup', 'API 组'),
      text('kind', '资源类型'),
      text('name', '资源名称'),
    ]),
  ]);
const peer = object('', '对端配置', [
  {
    ...labelSelector('podSelector', 'Pod 选择器'),
    help: '单独设置时匹配本命名空间；与命名空间选择器同时设置时，两个条件同时满足。',
  },
  {
    ...labelSelector('namespaceSelector', '命名空间选择器'),
    help: '空选择器 {} 匹配所有命名空间；只匹配命名空间，不代表标签相同的 Pod。',
  },
  {
    ...object('ipBlock', 'IP 网段', [text('cidr', 'CIDR'), strings('except', '排除 CIDR')]),
    help: 'IP 网段不能与 Pod / 命名空间选择器组合在同一对端。',
  },
]);
const policyPorts = list(
  'ports',
  '允许端口',
  object('', '端口规则', [
    text('protocol', '协议', ['TCP', 'UDP', 'SCTP']),
    port('port', '端口'),
    number('endPort', '结束端口'),
  ]),
);
export function resourceFields(kind: VisualKind, creating: boolean): Field[] {
  switch (kind) {
    case 'Job':
      return [
        ...metadataFields,
        object(
          'spec',
          'Job 配置',
          creating
            ? jobFields
            : jobFields.filter(
                (f) =>
                  ![
                    'template',
                    'completionMode',
                    'backoffLimitPerIndex',
                    'maxFailedIndexes',
                  ].includes(f.key),
              ),
          true,
        ),
      ];
    case 'CronJob':
      return [
        ...metadataFields,
        object(
          'spec',
          'CronJob 配置',
          [
            text('schedule', 'Cron 表达式'),
            text('timeZone', '时区'),
            text('concurrencyPolicy', '并发策略', ['Allow', 'Forbid', 'Replace']),
            bool('suspend', '暂停调度'),
            number('startingDeadlineSeconds', '错过调度截止秒数'),
            number('successfulJobsHistoryLimit', '成功历史保留数'),
            number('failedJobsHistoryLimit', '失败历史保留数'),
            object(
              'jobTemplate',
              'Job 模板',
              [
                object('metadata', 'Job 元数据', metadataFields),
                object('spec', 'Job 配置', jobFields, true),
              ],
              true,
            ),
          ],
          true,
        ),
      ];
    case 'Ingress':
      return [
        ...metadataFields,
        object(
          'spec',
          'Ingress 配置',
          [
            text('ingressClassName', 'IngressClass'),
            backend('defaultBackend', '默认后端'),
            list(
              'rules',
              '路由规则',
              object('', '路由配置', [
                text('host', '域名'),
                object(
                  'http',
                  'HTTP 路由',
                  [
                    list(
                      'paths',
                      '路径',
                      object('', '路径配置', [
                        text('path', '路径'),
                        text('pathType', '路径匹配', ['Prefix', 'Exact', 'ImplementationSpecific']),
                        backend('backend', '后端'),
                      ]),
                    ),
                  ],
                  true,
                ),
              ]),
            ),
            list(
              'tls',
              'TLS 配置',
              object('', 'TLS', [
                strings('hosts', 'TLS 域名'),
                text('secretName', 'TLS Secret 名称'),
              ]),
            ),
          ],
          true,
        ),
      ];
    case 'NetworkPolicy':
      return [
        ...metadataFields,
        object(
          'spec',
          '网络策略配置',
          [
            { ...labelSelector('podSelector', '目标 Pod'), required: true },
            {
              ...list('policyTypes', '策略方向', text('', '方向', ['Ingress', 'Egress'])),
              help: 'Ingress / Egress 表示隔离的方向；该方向没有规则时默认拒绝全部流量。',
            },
            {
              ...list(
                'ingress',
                '入站规则',
                object('', '入站配置', [
                  {
                    ...list('from', '允许来源', peer),
                    help: '未设置或空列表允许任意来源；列表内多项为“或”。',
                  },
                  policyPorts,
                ]),
              ),
              help: '空规则 {} 允许所有入站流量；空规则列表 [] 拒绝所有入站流量。',
            },
            {
              ...list(
                'egress',
                '出站规则',
                object('', '出站配置', [
                  {
                    ...list('to', '允许目标', peer),
                    help: '未设置或空列表允许任意目标；列表内多项为“或”。',
                  },
                  policyPorts,
                ]),
              ),
              help: '空规则 {} 允许所有出站流量；空规则列表 [] 拒绝所有出站流量。',
            },
          ],
          true,
        ),
      ];
    case 'Secret':
      return [
        ...metadataFields,
        text('type', 'Secret 类型', [
          'Opaque',
          'kubernetes.io/tls',
          'kubernetes.io/dockerconfigjson',
          'kubernetes.io/basic-auth',
          'kubernetes.io/ssh-auth',
          'kubernetes.io/service-account-token',
        ]),
        bool('immutable', '数据不可变'),
      ];
  }
}
