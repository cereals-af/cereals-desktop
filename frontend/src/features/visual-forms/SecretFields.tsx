import { convertSecretValue } from './secretEncoding';
import { useState } from 'react';
import { Alert, Button, Input, Select } from 'antd';
import type { SecretEntry } from '../../api/types';

export function SecretFields({
  entries,
  revealed,
  onChange,
  readOnly = false,
}: {
  entries: SecretEntry[];
  revealed: boolean;
  onChange: (entries: SecretEntry[]) => void;
  readOnly?: boolean;
}) {
  const [error, setError] = useState('');
  const [keys, setKeys] = useState(() => entries.map(() => crypto.randomUUID()));
  const set = (index: number, patch: Partial<SecretEntry>) =>
    onChange(entries.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)));
  return (
    <section className="container-section">
      <h4>配置数据</h4>
      {error && <Alert showIcon type="error" title={error} />}
      <p className="muted">
        文本以 UTF-8 解码；二进制保留 Base64。Base64 是编码，并非加密。TLS 使用 tls.crt /
        tls.key；镜像仓库使用 .dockerconfigjson。
      </p>
      {!revealed && <Alert type="info" showIcon title="内容已隐藏，点击“解码查看”后显示配置值。" />}
      {entries.map((entry, index) => (
        <section className="service-port" key={keys[index] ?? entry.key}>
          <div className="workload-fields">
            <label>
              键名
              <Input
                aria-label={`Secret 键名 ${index + 1}`}
                readOnly={readOnly}
                value={entry.key}
                onChange={(e) => set(index, { key: e.target.value })}
              />
            </label>
            {revealed && (
              <label>
                值格式
                <Select
                  aria-label={`Secret 格式 ${index + 1}`}
                  disabled={readOnly}
                  value={entry.encoding}
                  options={[
                    { value: 'text', label: '解码文本（UTF-8）' },
                    { value: 'base64', label: 'Base64（二进制）' },
                  ]}
                  onChange={(encoding) => {
                    try {
                      const converted = convertSecretValue(entry.value, entry.encoding, encoding);
                      set(index, { encoding, value: converted });
                      setError('');
                    } catch {
                      setError('当前内容不是有效 UTF-8 文本或 Base64，请检查格式。');
                    }
                  }}
                />
              </label>
            )}
          </div>
          {revealed ? (
            <Input.TextArea
              aria-label={`Secret 值 ${index + 1}`}
              autoSize={{ minRows: 3, maxRows: 12 }}
              readOnly={readOnly}
              value={entry.value}
              onChange={(e) => set(index, { value: e.target.value })}
            />
          ) : (
            <span className="muted">••••••••</span>
          )}
          {!readOnly && (
            <Button
              className="form-item-action"
              onClick={() => {
                setKeys(keys.filter((_, i) => i !== index));
                onChange(entries.filter((_, i) => i !== index));
              }}
            >
              移除此项
            </Button>
          )}
        </section>
      ))}
      {!readOnly && (
        <Button
          className="form-item-action"
          onClick={() => {
            setKeys([...keys, crypto.randomUUID()]);
            onChange([...entries, { key: '', value: '', encoding: 'text' }]);
          }}
        >
          添加配置项
        </Button>
      )}
    </section>
  );
}
