import { bool, choice, list, map, number, object, port, strings, text, type Field } from './schema';

const keyRef = (key: string, label: string) =>
  object(key, label, [text('name', '资源名称'), text('key', '键名'), bool('optional', '可选引用')]);
const environmentDefinition = list(
  'env',
  '环境变量',
  object('', '环境变量配置', [
    text('name', '变量名称'),
    choice('', '变量来源', [
      text('value', '直接赋值'),
      choice('valueFrom', '引用', [
        keyRef('secretKeyRef', 'Secret 键'),
        keyRef('configMapKeyRef', 'ConfigMap 键'),
        object('fieldRef', 'Pod 字段', [
          text('apiVersion', 'API 版本'),
          text('fieldPath', '字段路径'),
        ]),
        object('resourceFieldRef', '容器资源', [
          text('containerName', '容器名称'),
          text('resource', '资源字段', [
            'limits.cpu',
            'limits.memory',
            'requests.cpu',
            'requests.memory',
            'limits.ephemeral-storage',
            'requests.ephemeral-storage',
          ]),
          text('divisor', '除数'),
        ]),
      ]),
    ]),
  ]),
);
export const environment: Field = { ...environmentDefinition, editor: 'environment' };
const securityFields = [
  number('runAsUser', '运行 UID'),
  number('runAsGroup', '运行 GID'),
  bool('runAsNonRoot', '禁止 root'),
  object('seccompProfile', 'Seccomp', [
    text('type', '类型', ['RuntimeDefault', 'Localhost', 'Unconfined']),
    text('localhostProfile', '本地配置路径'),
  ]),
  object('seLinuxOptions', 'SELinux', [
    text('user', '用户'),
    text('role', '角色'),
    text('type', '类型'),
    text('level', '级别'),
  ]),
];
export const containerAdvanced: Field[] = [
  environment,
  {
    ...list(
      'envFrom',
      '环境变量批量引用',
      object('', '引用配置', [
        text('prefix', '变量前缀'),
        choice('', '引用来源', [
          object('configMapRef', 'ConfigMap', [text('name', '资源名称'), bool('optional', '可选')]),
          object('secretRef', 'Secret', [text('name', '资源名称'), bool('optional', '可选')]),
        ]),
      ]),
    ),
    editor: 'environmentImports',
  },
  list(
    'volumeMounts',
    '卷挂载',
    object('', '挂载配置', [
      text('name', '卷名称'),
      text('mountPath', '挂载路径'),
      text('subPath', '子路径'),
      text('subPathExpr', '子路径表达式'),
      bool('readOnly', '只读'),
      text('mountPropagation', '挂载传播', ['None', 'HostToContainer', 'Bidirectional']),
      text('recursiveReadOnly', '递归只读', ['Disabled', 'IfPossible', 'Enabled']),
    ]),
  ),
  list(
    'volumeDevices',
    '块设备',
    object('', '块设备配置', [text('name', '卷名称'), text('devicePath', '设备路径')]),
  ),
  object('securityContext', '容器安全上下文', [
    ...securityFields,
    bool('privileged', '特权容器'),
    bool('allowPrivilegeEscalation', '允许权限提升'),
    bool('readOnlyRootFilesystem', '只读根文件系统'),
    object('capabilities', 'Linux Capabilities', [
      strings('add', '添加能力'),
      strings('drop', '移除能力'),
    ]),
  ]),
  text('workingDir', '工作目录'),
  text('imagePullPolicy', '镜像拉取策略', ['Always', 'IfNotPresent', 'Never']),
];
const matchExpression = object('', '匹配表达式', [
  text('key', '标签键'),
  text('operator', '运算符', ['In', 'NotIn', 'Exists', 'DoesNotExist']),
  strings('values', '匹配值'),
]);
export const labelSelector = (key: string, label: string): Field => ({
  ...object(key, label, [
    map('matchLabels', '匹配标签'),
    { ...list('matchExpressions', '匹配表达式', matchExpression), editor: 'expressions' },
  ]),
  help: '空选择器 {} 匹配当前范围内所有对象。多个条件同时满足才匹配。',
});
const nodeTerm = object('', '节点匹配条件', [
  {
    ...list('matchExpressions', '节点标签表达式', {
      ...matchExpression,
      fields: matchExpression.fields?.map((f) =>
        f.key === 'operator'
          ? { ...f, options: ['In', 'NotIn', 'Exists', 'DoesNotExist', 'Gt', 'Lt'] }
          : f,
      ),
    }),
    editor: 'expressions',
  },
  { ...list('matchFields', '节点字段表达式', matchExpression), editor: 'expressions' },
]);
const affinityTerm = object('', 'Pod 亲和条件', [
  labelSelector('labelSelector', 'Pod 选择器'),
  strings('namespaces', '命名空间'),
  labelSelector('namespaceSelector', '命名空间选择器'),
  text('topologyKey', '拓扑键'),
  strings('matchLabelKeys', '匹配标签键'),
  strings('mismatchLabelKeys', '不匹配标签键'),
]);
const podAffinity = (key: string, label: string) =>
  object(key, label, [
    list('requiredDuringSchedulingIgnoredDuringExecution', '必须满足', affinityTerm),
    list(
      'preferredDuringSchedulingIgnoredDuringExecution',
      '优先满足',
      object('', '加权条件', [
        number('weight', '权重（1–100）'),
        { ...affinityTerm, key: 'podAffinityTerm', inline: true },
      ]),
    ),
  ]);
const keyPaths = list(
  'items',
  '键与文件映射',
  object('', '文件映射', [
    text('key', '键名'),
    text('path', '相对路径'),
    number('mode', '文件权限（十进制）'),
  ]),
);
const volumeSources = [
  object('emptyDir', '临时目录', [
    text('medium', '存储介质', ['', 'Memory']),
    text('sizeLimit', '容量限制'),
  ]),
  object('persistentVolumeClaim', 'PVC', [text('claimName', 'PVC 名称'), bool('readOnly', '只读')]),
  object('configMap', 'ConfigMap', [
    text('name', 'ConfigMap 名称'),
    bool('optional', '可选'),
    number('defaultMode', '默认权限（十进制）'),
    keyPaths,
  ]),
  object('secret', 'Secret', [
    text('secretName', 'Secret 名称'),
    bool('optional', '可选'),
    number('defaultMode', '默认权限（十进制）'),
    keyPaths,
  ]),
  object('hostPath', '主机目录', [
    text('path', '主机路径'),
    text('type', '路径类型', [
      '',
      'DirectoryOrCreate',
      'Directory',
      'FileOrCreate',
      'File',
      'Socket',
      'CharDevice',
      'BlockDevice',
    ]),
  ]),
  object('nfs', 'NFS', [text('server', '服务器'), text('path', '路径'), bool('readOnly', '只读')]),
  object('csi', 'CSI 内联卷', [
    text('driver', '驱动'),
    text('fsType', '文件系统'),
    bool('readOnly', '只读'),
    map('volumeAttributes', '卷属性'),
    object('nodePublishSecretRef', '节点凭据', [text('name', 'Secret 名称')]),
  ]),
  object('downwardAPI', 'Downward API', [
    number('defaultMode', '默认权限（十进制）'),
    list(
      'items',
      '字段文件',
      object('', '字段映射', [
        text('path', '文件路径'),
        number('mode', '文件权限'),
        object('fieldRef', 'Pod 字段', [text('fieldPath', '字段路径')]),
        object('resourceFieldRef', '资源字段', [
          text('containerName', '容器名称'),
          text('resource', '资源字段'),
          text('divisor', '除数'),
        ]),
      ]),
    ),
  ]),
];
const projected = object('projected', '投射卷', [
  number('defaultMode', '默认权限（十进制）'),
  list(
    'sources',
    '投射来源',
    choice('', '来源', [
      ...volumeSources.filter((f) => ['configMap', 'secret', 'downwardAPI'].includes(f.key)),
      object('serviceAccountToken', 'ServiceAccount Token', [
        text('audience', '受众'),
        number('expirationSeconds', '有效秒数'),
        text('path', '文件路径'),
      ]),
    ]),
  ),
]);
const probe = (key: string, label: string) =>
  object(key, label, [
    choice('', '探针方式', [
      object('httpGet', 'HTTP', [
        text('path', '路径'),
        port('port', '端口'),
        text('scheme', '协议', ['HTTP', 'HTTPS']),
        list(
          'httpHeaders',
          'HTTP 头',
          object('', '请求头', [text('name', '名称'), text('value', '值')]),
        ),
      ]),
      object('tcpSocket', 'TCP', [port('port', '端口')]),
      object('exec', '命令', [strings('command', '命令项')]),
      object('grpc', 'gRPC', [number('port', '端口'), text('service', '服务')]),
    ]),
    number('initialDelaySeconds', '初始延迟'),
    number('periodSeconds', '检查周期'),
    number('timeoutSeconds', '超时秒数'),
    number('failureThreshold', '失败阈值'),
    number('successThreshold', '成功阈值'),
  ]);
const containerDefinition = object('', '容器配置', [
  text('name', '容器名称'),
  text('image', '容器镜像'),
  strings('command', '启动命令'),
  strings('args', '启动参数'),
  {
    ...list(
      'ports',
      '端口映射',
      object('', '端口配置', [
        text('name', '端口名称'),
        number('containerPort', '容器端口'),
        text('protocol', '协议', ['TCP', 'UDP', 'SCTP']),
        number('hostPort', '主机端口'),
        text('hostIP', '主机 IP'),
      ]),
    ),
    inline: true,
    editor: 'ports',
  },
  object('resources', '资源配额', [map('requests', '请求量'), map('limits', '限制量')]),
  ...containerAdvanced,
  probe('readinessProbe', '就绪探针'),
  probe('livenessProbe', '存活探针'),
  probe('startupProbe', '启动探针'),
]);
export const container: Field = { ...containerDefinition, editor: 'container' };
export const podOptions: Field[] = [
  {
    ...list('initContainers', '初始化容器', {
      ...container,
      fields: [
        ...(container.fields ?? []),
        text('restartPolicy', '初始化容器重启策略', ['Always']),
      ],
    }),
    direct: true,
  },
  list(
    'volumes',
    '存储卷',
    object('', '卷配置', [
      text('name', '卷名称'),
      choice('', '卷来源', [...volumeSources, projected]),
    ]),
  ),
  { ...map('nodeSelector', '节点选择器'), inline: true },
  {
    ...object('affinity', '亲和性', [
      object('nodeAffinity', '节点亲和性', [
        object('requiredDuringSchedulingIgnoredDuringExecution', '必须满足', [
          list('nodeSelectorTerms', '节点条件（或）', nodeTerm),
        ]),
        list(
          'preferredDuringSchedulingIgnoredDuringExecution',
          '优先满足',
          object('', '加权条件', [
            number('weight', '权重（1–100）'),
            { ...nodeTerm, key: 'preference', inline: true },
          ]),
        ),
      ]),
      podAffinity('podAffinity', 'Pod 亲和性'),
      podAffinity('podAntiAffinity', 'Pod 反亲和性'),
    ]),
    editor: 'affinity',
  },
  list(
    'tolerations',
    '污点容忍',
    object('', '容忍配置', [
      text('key', '污点键'),
      text('operator', '运算符', ['Equal', 'Exists']),
      text('value', '污点值'),
      text('effect', '作用', ['NoSchedule', 'PreferNoSchedule', 'NoExecute']),
      number('tolerationSeconds', '容忍秒数'),
    ]),
  ),
  object('securityContext', 'Pod 安全上下文', [
    ...securityFields,
    number('fsGroup', '文件系统组'),
    text('fsGroupChangePolicy', '文件系统组策略', ['OnRootMismatch', 'Always']),
    list('supplementalGroups', '附加组', number('', '组 ID')),
    list('sysctls', '内核参数', object('', '参数', [text('name', '名称'), text('value', '值')])),
  ]),
  text('serviceAccountName', 'ServiceAccount'),
  bool('automountServiceAccountToken', '自动挂载 ServiceAccount Token'),
  list('imagePullSecrets', '镜像仓库凭据', object('', '凭据', [text('name', 'Secret 名称')])),
  number('terminationGracePeriodSeconds', '终止宽限秒数'),
  text('priorityClassName', 'PriorityClass'),
  bool('hostNetwork', '主机网络'),
  bool('hostPID', '主机 PID'),
  bool('hostIPC', '主机 IPC'),
  text('dnsPolicy', 'DNS 策略', ['ClusterFirst', 'ClusterFirstWithHostNet', 'Default', 'None']),
  object('dnsConfig', 'DNS 配置', [
    strings('nameservers', 'DNS 服务器'),
    strings('searches', '搜索域'),
    list('options', 'DNS 选项', object('', '选项', [text('name', '名称'), text('value', '值')])),
  ]),
  list(
    'topologySpreadConstraints',
    '拓扑分布',
    object('', '分布规则', [
      number('maxSkew', '最大偏斜'),
      text('topologyKey', '拓扑键'),
      text('whenUnsatisfiable', '不满足时', ['DoNotSchedule', 'ScheduleAnyway']),
      labelSelector('labelSelector', 'Pod 选择器'),
      number('minDomains', '最少域数量'),
      text('nodeAffinityPolicy', '节点亲和策略', ['Honor', 'Ignore']),
      text('nodeTaintsPolicy', '污点策略', ['Honor', 'Ignore']),
      strings('matchLabelKeys', '匹配标签键'),
    ]),
  ),
];
export const podTemplate = object(
  'template',
  'Pod 模板',
  [
    object('metadata', 'Pod 元数据', [map('labels', '标签'), map('annotations', '注解')]),
    {
      ...object(
        'spec',
        'Pod 配置',
        [
          { ...list('containers', '容器', container), required: true },
          text('restartPolicy', '重启策略', ['Never', 'OnFailure']),
          ...podOptions,
        ],
        true,
      ),
      editor: 'pod',
      inline: true,
    },
  ],
  true,
);
