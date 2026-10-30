import { useState } from 'react';
import { Button, Input } from 'antd';

interface Process {
  command: string[];
  args: string[];
}

export function ContainerProcess({
  command,
  args,
  onChange,
}: Process & {
  onChange: (patch: Partial<Process>) => void;
}) {
  return (
    <section className="container-section container-process">
      <div className="container-section-heading">
        <h4>启动命令与参数</h4>
        <p className="muted">
          每项作为一个完整字符串传入，保留空格和换行，不自动拆分或添加引号。命令和参数均不设置时使用镜像默认启动配置。
        </p>
      </div>
      <ProcessList
        label="启动命令 command"
        itemLabel="命令项"
        values={command}
        onChange={(command) => onChange({ command })}
      />
      <ProcessList
        label="启动参数 args"
        itemLabel="参数"
        values={args}
        onChange={(args) => onChange({ args })}
      />
      <p className="muted">
        command 对应 ENTRYPOINT，args 对应 CMD。需要 shell 时，可依次填写命令项
        /bin/sh、-c，并将完整脚本填为一个参数。
      </p>
    </section>
  );
}

function ProcessList({
  label,
  itemLabel,
  values,
  onChange,
}: {
  label: string;
  itemLabel: string;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const [keys, setKeys] = useState(() => values.map(() => crypto.randomUUID()));
  return (
    <div className="process-list" role="group" aria-label={label}>
      <h4>{label}</h4>
      {values.map((value, index) => (
        <div className="process-row" key={keys[index]}>
          <Input.TextArea
            aria-label={`${itemLabel} ${index + 1}`}
            value={value}
            autoSize={{ minRows: 1, maxRows: 5 }}
            placeholder={
              itemLabel === '参数'
                ? '一个完整参数；空值表示空字符串'
                : index === 0
                  ? '例如 /bin/sh'
                  : '例如 -c'
            }
            onChange={(event) =>
              onChange(values.map((row, i) => (i === index ? event.target.value : row)))
            }
          />
          <Button
            className="form-item-action"
            onClick={() => {
              setKeys(keys.filter((_, i) => i !== index));
              onChange(values.filter((_, i) => i !== index));
            }}
          >
            移除此{itemLabel}
          </Button>
        </div>
      ))}
      <Button
        className="form-item-action"
        onClick={() => {
          setKeys([...keys, crypto.randomUUID()]);
          onChange([...values, '']);
        }}
      >
        添加{itemLabel}
      </Button>
    </div>
  );
}
