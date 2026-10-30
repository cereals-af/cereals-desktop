import type { ReactNode } from 'react';
import { Tabs } from 'antd';
import type { FieldsProps } from './schema';

import type { FieldGroup } from './groupDefinitions';

export function FieldGroups({
  groups,
  renderFields,
  leading,
  ...props
}: FieldsProps & {
  groups: FieldGroup[];
  renderFields: (props: FieldsProps) => ReactNode;
  leading?: Partial<Record<string, ReactNode>>;
}) {
  return (
    <Tabs
      className="configuration-tabs"
      items={groups.map((group) => ({
        key: group.key,
        label: group.label,
        children: (
          <>
            {leading?.[group.key]}
            {renderFields({
              ...props,
              fields: props.fields.filter((field) => group.fields.includes(field.key)),
            })}
          </>
        ),
      }))}
    />
  );
}
