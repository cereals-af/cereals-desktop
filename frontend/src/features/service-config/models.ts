import type {
  ConfigMapForm,
  ConfigMapSnapshot,
  DesktopAPI,
  Ref,
  ResourceType,
  ServiceForm,
  ServicePort,
  ServiceSnapshot,
} from '../../api/types';

export type FormKind = 'service' | 'configmap';
export type FormDocument =
  | { kind: 'service'; snapshot: ServiceSnapshot }
  | { kind: 'configmap'; snapshot: ConfigMapSnapshot };

export function coreFormKind(type: ResourceType): FormKind | undefined {
  if (type.group || type.version !== 'v1') return undefined;
  if (type.resource === 'services') return 'service';
  if (type.resource === 'configmaps') return 'configmap';
  return undefined;
}

export const newPort = (): ServicePort => ({
  originalKey: '',
  name: 'http',
  protocol: 'TCP',
  port: 80,
  targetPort: '8080',
  nodePort: 0,
  appProtocol: '',
});
export const newService = (namespace: string): ServiceForm => ({
  name: '',
  namespace,
  type: 'ClusterIP',
  headless: false,
  externalName: '',
  selector: [],
  ports: [newPort()],
  externalTrafficPolicy: 'Cluster',
  sessionAffinity: 'None',
  publishNotReadyAddresses: false,
});
export const newConfigMap = (namespace: string): ConfigMapForm => ({
  name: '',
  namespace,
  data: [],
  binaryData: [],
  immutable: false,
});

export function newDocument(kind: FormKind, namespace: string): FormDocument {
  const baseline = { uid: '', resourceVersion: '' };
  return kind === 'service'
    ? {
        kind,
        snapshot: {
          form: newService(namespace),
          baseline,
          clusterIPs: [],
          ipFamilies: [],
          loadBalancerAddresses: [],
        },
      }
    : { kind, snapshot: { form: newConfigMap(namespace), baseline } };
}

export async function loadDocument(
  api: DesktopAPI,
  kind: FormKind,
  target: Ref,
  signal: AbortSignal,
): Promise<FormDocument> {
  return kind === 'service'
    ? { kind, snapshot: await api.service(target, signal) }
    : { kind, snapshot: await api.configMap(target, signal) };
}
