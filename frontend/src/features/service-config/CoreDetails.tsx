import { Alert, Collapse, Descriptions, Table, Tag } from 'antd';
import type { FormDocument } from './models';

export function CoreDetails({ document }: { document: FormDocument }) {
  if (document.kind === 'configmap') {
    const { form } = document.snapshot;
    return (
      <>
        <Descriptions
          items={[
            { key: 'text', label: '文本键', children: form.data?.length ?? 0 },
            { key: 'binary', label: '二进制键', children: form.binaryData?.length ?? 0 },
            {
              key: 'immutable',
              label: '不可变',
              children: (
                <Tag color={form.immutable ? 'warning' : 'default'}>
                  {form.immutable ? '已锁定' : '否'}
                </Tag>
              ),
            },
          ]}
        />
        <Alert
          showIcon
          type="info"
          title="更新 ConfigMap 不会自动重启应用；环境变量或 subPath 挂载等用法需要重新创建 Pod 才能读取新配置。"
        />
        <h3>文本数据</h3>
        <Collapse
          items={(form.data ?? []).map((entry) => ({
            key: entry.key,
            label: entry.key,
            children: <pre className="config-preview">{entry.value || '（空字符串）'}</pre>,
          }))}
        />
        <h3>二进制数据（Base64）</h3>
        <Collapse
          items={(form.binaryData ?? []).map((entry) => ({
            key: entry.key,
            label: entry.key,
            children: <pre className="config-preview">{entry.value || '（空数据）'}</pre>,
          }))}
        />
      </>
    );
  }
  const { form, clusterIPs, ipFamilies, loadBalancerAddresses } = document.snapshot;
  return (
    <>
      <dl className="service-summary">
        {[
          {
            label: '类型',
            value: <Tag color="success">{form.headless ? 'Headless' : form.type}</Tag>,
          },
          { label: '集群 IP', value: clusterIPs?.join(', ') || '—' },
          { label: 'IP 协议族', value: ipFamilies?.join(', ') || '—' },
          {
            label: '外部地址 / DNS',
            value:
              form.type === 'ExternalName'
                ? form.externalName
                : loadBalancerAddresses?.join(', ') ||
                  (form.type === 'LoadBalancer' ? '等待分配' : '—'),
          },
          { label: '会话保持', value: form.sessionAffinity },
          { label: '外部流量策略', value: form.externalTrafficPolicy || '—' },
        ].map((item) => (
          <div key={item.label}>
            <dt>{item.label}</dt>
            <dd>{item.value}</dd>
          </div>
        ))}
      </dl>
      <h3>Pod 选择器</h3>
      {(form.selector ?? []).map((entry) => (
        <Tag key={entry.key}>
          {entry.key}={entry.value}
        </Tag>
      ))}
      {!form.selector?.length && <p className="muted">未配置选择器，不会自动选择 Pod。</p>}
      <h3>端口映射</h3>
      <Table
        size="small"
        rowKey="originalKey"
        pagination={false}
        dataSource={form.ports ?? []}
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '协议', dataIndex: 'protocol' },
          { title: 'Service 端口', dataIndex: 'port' },
          { title: '目标端口', dataIndex: 'targetPort' },
          { title: '节点端口', key: 'nodePort', render: (_, port) => port.nodePort || '—' },
          { title: '应用协议', dataIndex: 'appProtocol' },
        ]}
      />
      {form.type === 'LoadBalancer' && (
        <Alert
          className="form-section"
          showIcon
          type="info"
          title="外部地址由集群负载均衡控制器分配；可刷新详情查看最新状态。"
        />
      )}
    </>
  );
}
