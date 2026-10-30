export function supportsScale(target: { group: string; version: string; resource: string }) {
  return (
    target.version === 'v1' &&
    ((target.group === 'apps' &&
      ['deployments', 'statefulsets', 'replicasets'].includes(target.resource)) ||
      (target.group === '' && target.resource === 'replicationcontrollers'))
  );
}
