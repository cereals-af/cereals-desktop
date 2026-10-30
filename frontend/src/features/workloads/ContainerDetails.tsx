import { Collapse, Descriptions, Table } from 'antd';
import { Fields } from '../visual-forms/Fields';
import { containerAdvanced, podOptions as podFields } from '../visual-forms/podSchema';
import type { CustomValue, Probe, WorkloadContainer } from '../../api/types';

export function ContainerDetails({
  containers,
  podOptions,
}: {
  containers: WorkloadContainer[] | null;
  podOptions?: CustomValue | null;
}) {
  return (
    <>
      {podOptions && (
        <Collapse
          items={[
            {
              key: 'pod',
              label: 'Pod 高级配置',
              children: (
                <Fields fields={podFields} value={podOptions} onChange={() => undefined} readOnly />
              ),
            },
          ]}
        />
      )}
      {(containers ?? []).map((container) => (
        <section className="workload-container" key={container.name}>
          <h3>容器 · {container.name}</h3>
          <Descriptions
            size="small"
            column={2}
            items={[
              {
                key: 'image',
                label: '镜像',
                span: 2,
                children: <code className="config-preview">{container.image}</code>,
              },
              { key: 'cpuRequest', label: 'CPU 请求', children: container.cpuRequest || '未设置' },
              { key: 'cpuLimit', label: 'CPU 上限', children: container.cpuLimit || '未设置' },
              {
                key: 'memoryRequest',
                label: '内存请求',
                children: container.memoryRequest || '未设置',
              },
              {
                key: 'memoryLimit',
                label: '内存上限',
                children: container.memoryLimit || '未设置',
              },
              {
                key: 'command',
                label: '启动命令 command',
                span: 2,
                children: (
                  <code className="config-preview">
                    {container.command?.length
                      ? JSON.stringify(container.command)
                      : '使用镜像默认命令'}
                  </code>
                ),
              },
              {
                key: 'args',
                label: '启动参数 args',
                span: 2,
                children: (
                  <code className="config-preview">
                    {container.args?.length
                      ? JSON.stringify(container.args)
                      : container.command?.length
                        ? '未设置额外参数'
                        : '使用镜像默认参数'}
                  </code>
                ),
              },
            ]}
          />
          <h3>端口映射</h3>
          <Table
            size="small"
            rowKey="originalKey"
            dataSource={container.ports ?? []}
            pagination={false}
            locale={{ emptyText: '未声明容器端口' }}
            columns={[
              { title: '名称', render: (_, port) => port.name || '—' },
              { title: '协议', dataIndex: 'protocol' },
              { title: '容器端口', dataIndex: 'containerPort' },
              { title: '主机端口', render: (_, port) => port.hostPort || '未映射' },
              { title: '主机 IP', render: (_, port) => port.hostIP || '默认地址' },
            ]}
          />
          <h3>环境变量</h3>
          <Table
            size="small"
            rowKey="name"
            dataSource={container.environment ?? []}
            pagination={false}
            locale={{ emptyText: '没有显式环境变量；批量引用可在高级配置中查看' }}
            columns={[
              { title: '名称', dataIndex: 'name', width: 200 },
              {
                title: '值 / 引用',
                render: (_, env) => (
                  <code className="config-preview">{env.source || env.value || '空字符串'}</code>
                ),
              },
            ]}
          />
          {container.advanced && (
            <Collapse
              items={[
                {
                  key: 'advanced',
                  label: '环境变量引用、卷挂载与安全配置',
                  children: (
                    <Fields
                      fields={containerAdvanced}
                      value={container.advanced}
                      onChange={() => undefined}
                      readOnly
                    />
                  ),
                },
              ]}
            />
          )}
          <h3>健康检查</h3>
          <Collapse
            items={(
              [
                ['readiness', '就绪探针', container.readiness],
                ['liveness', '存活探针', container.liveness],
                ['startup', '启动探针', container.startup],
              ] as const
            ).map(([key, label, probe]) => ({
              key,
              label: `${label} · ${probe.kind === 'none' ? '未设置' : probe.kind}`,
              children: <ProbeDetails probe={probe} />,
            }))}
          />
        </section>
      ))}
    </>
  );
}
function ProbeDetails({ probe }: { probe: Probe }) {
  if (probe.kind === 'none') return <p className="muted">未设置此探针。</p>;
  if (probe.kind === 'keep')
    return <p className="muted">高级探针配置保留在原始资源中，请查看 YAML。</p>;
  const handler =
    probe.kind === 'exec'
      ? probe.command?.join(' ')
      : probe.kind === 'http'
        ? `${probe.scheme} :${probe.port}${probe.path}`
        : `TCP :${probe.port}`;
  return (
    <Descriptions
      size="small"
      column={2}
      items={[
        {
          key: 'handler',
          label: '检查方式',
          span: 2,
          children: <code className="config-preview">{handler}</code>,
        },
        { key: 'initial', label: '初始延迟', children: `${probe.initialDelay} 秒` },
        { key: 'timeout', label: '超时', children: `${probe.timeout} 秒` },
        { key: 'period', label: '周期', children: `${probe.period} 秒` },
        { key: 'success', label: '成功阈值', children: probe.successThreshold },
        { key: 'failure', label: '失败阈值', children: probe.failureThreshold },
      ]}
    />
  );
}
