export interface FieldGroup {
  key: string;
  label: string;
  fields: string[];
}

export const containerGroups: FieldGroup[] = [
  {
    key: 'basic',
    label: '基础配置',
    fields: ['resources', 'command', 'args', 'workingDir', 'imagePullPolicy', 'restartPolicy'],
  },
  { key: 'ports', label: '端口', fields: ['ports'] },
  { key: 'environment', label: '环境变量', fields: ['env', 'envFrom'] },
  { key: 'storage', label: '存储挂载', fields: ['volumeMounts', 'volumeDevices'] },
  { key: 'health', label: '健康检查', fields: ['readinessProbe', 'livenessProbe', 'startupProbe'] },
  { key: 'security', label: '安全设置', fields: ['securityContext'] },
];

export const podGroups: FieldGroup[] = [
  {
    key: 'scheduling',
    label: '调度配置',
    fields: [
      'nodeSelector',
      'affinity',
      'tolerations',
      'topologySpreadConstraints',
      'priorityClassName',
    ],
  },
  { key: 'init', label: '初始化容器', fields: ['initContainers'] },
  { key: 'volumes', label: '存储卷', fields: ['volumes'] },
  {
    key: 'security',
    label: '安全与凭据',
    fields: [
      'securityContext',
      'serviceAccountName',
      'automountServiceAccountToken',
      'imagePullSecrets',
    ],
  },
  {
    key: 'network',
    label: '网络与 DNS',
    fields: ['hostNetwork', 'hostPID', 'hostIPC', 'dnsPolicy', 'dnsConfig'],
  },
  {
    key: 'lifecycle',
    label: '生命周期',
    fields: ['restartPolicy', 'terminationGracePeriodSeconds'],
  },
];
