import { Input } from 'antd';
import { NamespaceSelect } from '../resources/NamespaceSelect';

export function IdentityFields({
  kind,
  clusterID,
  name,
  namespace,
  onName,
  onNamespace,
}: {
  kind: string;
  clusterID: string;
  name: string;
  namespace: string;
  onName: (value: string) => void;
  onNamespace: (value: string) => void;
}) {
  return (
    <div className="workload-fields">
      <label>
        {kind} 名称
        <Input
          aria-label={`${kind} 名称`}
          value={name}
          onChange={(event) => onName(event.target.value)}
        />
      </label>
      <label>
        命名空间
        <NamespaceSelect
          clusterID={clusterID}
          allowAll={false}
          value={namespace}
          onChange={onNamespace}
        />
      </label>
    </div>
  );
}
