import { Alert, Button, Checkbox, ConfigProvider, Input, Tabs } from 'antd';
import type { ConfigEntry, ConfigMapForm } from '../../api/types';
import { IdentityFields } from './IdentityFields';

export function ConfigMapFields({
  value,
  originalImmutable,
  creating,
  clusterID,
  onChange,
}: {
  value: ConfigMapForm;
  originalImmutable: boolean;
  creating: boolean;
  clusterID: string;
  onChange: (value: ConfigMapForm) => void;
}) {
  const set = (patch: Partial<ConfigMapForm>) => onChange({ ...value, ...patch });
  return (
    <>
      {creating && (
        <IdentityFields
          kind="ConfigMap"
          clusterID={clusterID}
          name={value.name}
          namespace={value.namespace}
          onName={(name) => set({ name })}
          onNamespace={(namespace) => set({ namespace })}
        />
      )}
      <Alert
        showIcon
        type="info"
        title="ConfigMap 存储非敏感配置，文本和解码后二进制数据合计最多 1 MiB。修改配置不会自动重启应用。"
      />
      <div className="form-section">
        <Checkbox
          disabled={originalImmutable}
          checked={value.immutable}
          onChange={(event) => set({ immutable: event.target.checked })}
        >
          设为不可变（提交后不能再修改数据或取消此设置）
        </Checkbox>
      </div>
      {originalImmutable ? (
        <Alert
          showIcon
          type="warning"
          title="该 ConfigMap 已不可变，数据只能查看。需要修改时请创建新的 ConfigMap。"
        />
      ) : (
        value.immutable && (
          <Alert
            showIcon
            type="warning"
            title="确认提交后，当前数据将被永久锁定；请检查差异后再提交。"
          />
        )
      )}
      <ConfigProvider componentDisabled={originalImmutable || undefined}>
        <Tabs
          items={[
            {
              key: 'text',
              label: `文本数据 · ${value.data?.length ?? 0}`,
              children: (
                <ConfigEntries
                  entries={value.data ?? []}
                  binary={false}
                  onChange={(data) => set({ data })}
                />
              ),
            },
            {
              key: 'binary',
              label: `二进制数据 · ${value.binaryData?.length ?? 0}`,
              children: (
                <ConfigEntries
                  entries={value.binaryData ?? []}
                  binary
                  onChange={(binaryData) => set({ binaryData })}
                />
              ),
            },
          ]}
        />
      </ConfigProvider>
    </>
  );
}

function ConfigEntries({
  entries,
  binary,
  onChange,
}: {
  entries: ConfigEntry[];
  binary: boolean;
  onChange: (entries: ConfigEntry[]) => void;
}) {
  const label = binary ? '二进制' : '文本';
  const update = (index: number, patch: Partial<ConfigEntry>) =>
    onChange(entries.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)));
  return (
    <div className="config-entries">
      {binary && (
        <p className="workload-note muted">
          填写标准 Base64；与文本键名不能重复。值在后端校验，不会自动转换编码。
        </p>
      )}
      {entries.map((entry, index) => (
        <section className="config-entry" key={index}>
          <div className="config-entry-header">
            <label>
              键名
              <Input
                aria-label={`${label}键名 ${index + 1}`}
                value={entry.key}
                placeholder={binary ? 'payload.bin' : 'application.yaml'}
                onChange={(event) => update(index, { key: event.target.value })}
              />
            </label>
            <Button onClick={() => onChange(entries.filter((_, i) => i !== index))}>
              移除此配置
            </Button>
          </div>
          <label>
            {binary ? 'Base64 内容' : '配置内容'}
            <Input.TextArea
              aria-label={`${label}内容 ${index + 1}`}
              spellCheck={false}
              autoSize={{ minRows: 4, maxRows: 16 }}
              value={entry.value}
              onChange={(event) => update(index, { value: event.target.value })}
            />
          </label>
        </section>
      ))}
      <Button onClick={() => onChange([...entries, { key: '', value: '' }])}>
        添加{label}配置
      </Button>
    </div>
  );
}
