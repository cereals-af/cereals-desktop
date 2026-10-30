import { Alert, Input } from 'antd';
import type { CustomValue, VisualSnapshot } from '../../api/types';
import { property, setProperty, valueOf } from '../custom-resources/values';
import { NamespaceSelect } from '../resources/NamespaceSelect';
import { Fields } from './Fields';
import { resourceFields, type VisualKind } from './resourceSchema';
import { SecretFields } from './SecretFields';

export function ResourceFields({
  snapshot,
  kind,
  creating,
  clusterID,
  onChange,
  readOnly = false,
}: {
  snapshot: VisualSnapshot;
  kind: VisualKind;
  creating: boolean;
  clusterID: string;
  onChange: (value: VisualSnapshot) => void;
  readOnly?: boolean;
}) {
  const set = (value: CustomValue) => onChange({ ...snapshot, value });
  return (
    <>
      {creating && (
        <div className="workload-fields">
          <label>
            {kind} 名称
            <Input
              aria-label={`${kind} 名称`}
              value={property(snapshot.value, 'name')?.text}
              onChange={(e) =>
                set(setProperty(snapshot.value, 'name', valueOf('string', e.target.value)))
              }
            />
          </label>
          <label>
            命名空间
            <NamespaceSelect
              clusterID={clusterID}
              allowAll={false}
              value={property(snapshot.value, 'namespace')?.text ?? ''}
              onChange={(namespace) =>
                set(setProperty(snapshot.value, 'namespace', valueOf('string', namespace)))
              }
            />
          </label>
        </div>
      )}
      {kind === 'Job' && !creating && !readOnly && (
        <Alert
          type="info"
          showIcon
          title="已有 Job 的 Pod 模板不可直接修改。可调整执行参数或暂停 / 恢复；新模板请创建新 Job。集群将校验其他不可变字段。"
        />
      )}
      {kind === 'NetworkPolicy' && (
        <Alert
          type="info"
          showIcon
          title="网络策略需要集群网络插件支持；请确认目标 Pod、流量方向及规则范围后提交。"
        />
      )}
      <Fields
        fields={resourceFields(kind, creating || readOnly)}
        value={snapshot.value}
        onChange={set}
        readOnly={readOnly}
      />
      {kind === 'Secret' && (
        <SecretFields
          entries={snapshot.entries ?? []}
          revealed={snapshot.revealed}
          readOnly={readOnly}
          onChange={(entries) => onChange({ ...snapshot, entries })}
        />
      )}
    </>
  );
}
