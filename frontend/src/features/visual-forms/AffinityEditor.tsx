import { Fragment, useState, type ReactNode } from 'react';
import { Select } from 'antd';
import type { CustomValue } from '../../api/types';
import { setProperty, valueOf } from '../custom-resources/values';
import {
  affinityRules,
  affinityTypes,
  preferredAffinity,
  requiredAffinity,
  setAffinityRules,
} from './affinity';
import type { Field, FieldsProps } from './schema';

export function AffinityEditor({
  value,
  onChange,
  path,
  field,
  renderFields,
}: {
  value: CustomValue;
  onChange: (value: CustomValue) => void;
  path: string;
  field: Field;
  renderFields: (props: FieldsProps) => ReactNode;
}) {
  const [type, setType] = useState(
    () => affinityTypes.find((item) => value.properties?.[item.value])?.value ?? 'nodeAffinity',
  );
  const definition = field.fields?.find((item) => item.key === type);
  const label = affinityTypes.find((item) => item.value === type)?.label ?? '';
  return (
    <div className="affinity-editor">
      <label className="visual-field affinity-type">
        <span>亲和性类型</span>
        <Select
          aria-label={`${path} / 亲和性类型`}
          value={type}
          onChange={setType}
          options={affinityTypes.map((item) => {
            const count = [requiredAffinity, preferredAffinity].reduce(
              (sum, mode) => sum + (affinityRules(value, item.value, mode)?.items?.length ?? 0),
              0,
            );
            return {
              value: item.value,
              label: `${item.label}${count ? ` · ${count} 条规则` : ''}`,
            };
          })}
        />
      </label>
      <p className="muted">
        {type === 'nodeAffinity'
          ? '按节点标签或字段选择调度位置。多个必须满足的规则组之间为“或”，组内条件为“且”。'
          : `按 Pod 标签${type === 'podAntiAffinity' ? '避开' : '选择'}调度位置。必须满足的规则之间为“且”；优先满足按权重影响调度。`}
      </p>
      {[requiredAffinity, preferredAffinity].map((mode) => {
        const source = definition?.fields?.find((item) => item.key === mode);
        const rulesField =
          type === 'nodeAffinity' && mode === requiredAffinity
            ? source?.fields?.find((item) => item.key === 'nodeSelectorTerms')
            : source;
        if (!rulesField) return null;
        const rules = affinityRules(value, type, mode);
        return (
          <Fragment key={`${type}-${mode}`}>
            {renderFields({
              fields: [
                {
                  ...rulesField,
                  key: mode,
                  label: mode === requiredAffinity ? '必须满足' : '优先满足',
                  inline: true,
                  help:
                    mode === requiredAffinity
                      ? '硬性约束，不满足时 Pod 无法调度。'
                      : '软性偏好，无法满足时仍可调度；权重范围 1–100。',
                },
              ],
              value: setProperty(valueOf('object'), mode, rules),
              path: `${path} / ${label}`,
              onChange: (next) =>
                onChange(setAffinityRules(value, type, mode, next.properties?.[mode])),
            })}
          </Fragment>
        );
      })}
    </div>
  );
}
