import type { Deployment, Probe, WorkloadContainer } from '../../api/types';

export function emptyProbe(): Probe {
  return {
    kind: 'none',
    path: '/',
    port: '',
    scheme: 'HTTP',
    command: [],
    initialDelay: 0,
    timeout: 1,
    period: 10,
    successThreshold: 1,
    failureThreshold: 3,
  };
}

export function emptyContainer(): WorkloadContainer {
  return {
    advanced: null,
    originalName: '',
    name: 'app',
    image: '',
    command: [],
    args: [],
    ports: [],
    cpuRequest: '',
    cpuLimit: '',
    memoryRequest: '',
    memoryLimit: '',
    environment: [],
    readiness: emptyProbe(),
    liveness: emptyProbe(),
    startup: emptyProbe(),
  };
}

export function emptyDeployment(namespace: string): Deployment {
  return { podOptions: null, name: '', namespace, replicas: 1, containers: [emptyContainer()] };
}
