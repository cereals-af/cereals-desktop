import type { WorkloadContainer } from '../api/types';

// A committed form is read back with identities assigned by the backend snapshot.
export function containerSnapshot(containers: WorkloadContainer[] | null): WorkloadContainer[] {
  return (containers ?? []).map((container) => ({
    ...container,
    originalName: container.name,
    ports: (container.ports ?? []).map((port) => ({
      ...port,
      originalKey: port.name ? `name:${port.name}` : `port:${port.containerPort}/${port.protocol}`,
    })),
  }));
}
