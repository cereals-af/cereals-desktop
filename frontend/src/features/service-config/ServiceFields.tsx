import { Alert, Button, Checkbox, Input, Select } from 'antd';
import type { ServiceForm } from '../../api/types';
import { IdentityFields } from './IdentityFields';
import { ServicePorts } from './ServicePorts';

export function ServiceFields({
  value,
  original,
  creating,
  clusterID,
  onChange,
}: {
  value: ServiceForm;
  original: ServiceForm;
  creating: boolean;
  clusterID: string;
  onChange: (value: ServiceForm) => void;
}) {
  const set = (patch: Partial<ServiceForm>) => onChange({ ...value, ...patch });
  const selector = value.selector ?? [];
  return (
    <>
      {creating && (
        <IdentityFields
          kind="Service"
          clusterID={clusterID}
          name={value.name}
          namespace={value.namespace}
          onName={(name) => set({ name })}
          onNamespace={(namespace) => set({ namespace })}
        />
      )}
      <div className="workload-fields">
        <label>
          服务类型
          <Select
            aria-label="服务类型"
            value={value.type}
            onChange={(type) =>
              set({ type, headless: type === 'ClusterIP' ? original.headless : false })
            }
            options={['ClusterIP', 'NodePort', 'LoadBalancer', 'ExternalName'].map((type) => ({
              value: type,
              label: type,
            }))}
          />
        </label>
        {value.type === 'ExternalName' ? (
          <label>
            外部 DNS 名称
            <Input
              aria-label="外部 DNS 名称"
              value={value.externalName}
              onChange={(event) => set({ externalName: event.target.value })}
              placeholder="api.example.com"
            />
          </label>
        ) : (
          <label>
            会话保持
            <Select
              aria-label="会话保持"
              value={value.sessionAffinity}
              onChange={(sessionAffinity) => set({ sessionAffinity })}
              options={[
                { value: 'None', label: 'None（不保持）' },
                { value: 'ClientIP', label: 'ClientIP（客户端 IP）' },
              ]}
            />
          </label>
        )}
        {(value.type === 'NodePort' || value.type === 'LoadBalancer') && (
          <label>
            外部流量策略
            <Select
              aria-label="外部流量策略"
              value={value.externalTrafficPolicy || 'Cluster'}
              onChange={(externalTrafficPolicy) => set({ externalTrafficPolicy })}
              options={['Cluster', 'Local'].map((type) => ({ value: type, label: type }))}
            />
          </label>
        )}
      </div>
      {!creating && value.type !== original.type && (
        <Alert
          showIcon
          type="warning"
          title="切换类型会清理不再适用的分配字段和策略，请仔细查看预检差异。"
        />
      )}
      {value.type === 'ExternalName' ? (
        <Alert
          showIcon
          type="info"
          title="ExternalName 提供 DNS 别名；不创建代理或选择后端 Pod。切换到此类型会移除原选择器、端口和集群 IP 配置。"
        />
      ) : (
        <>
          {value.type === 'ClusterIP' && (
            <Checkbox
              disabled={!creating && original.type !== 'ExternalName'}
              checked={value.headless}
              onChange={(event) => set({ headless: event.target.checked })}
            >
              无头服务（clusterIP: None；现有 IP 分配模式不能更改）
            </Checkbox>
          )}
          <section className="form-section">
            <h3>Pod 标签选择器</h3>
            {selector.map((entry, index) => (
              <div className="environment-row" key={index}>
                <Input
                  aria-label={`选择器键 ${index + 1}`}
                  value={entry.key}
                  placeholder="app"
                  onChange={(event) =>
                    set({
                      selector: selector.map((row, i) =>
                        i === index ? { ...row, key: event.target.value } : row,
                      ),
                    })
                  }
                />
                <Input
                  aria-label={`选择器值 ${index + 1}`}
                  value={entry.value}
                  placeholder="api-gateway"
                  onChange={(event) =>
                    set({
                      selector: selector.map((row, i) =>
                        i === index ? { ...row, value: event.target.value } : row,
                      ),
                    })
                  }
                />
                <Button onClick={() => set({ selector: selector.filter((_, i) => i !== index) })}>
                  移除
                </Button>
              </div>
            ))}
            <Button
              size="small"
              onClick={() => set({ selector: [...selector, { key: '', value: '' }] })}
            >
              添加选择器
            </Button>
            {selector.length === 0 && (
              <p className="workload-note muted">
                没有选择器，需要自行维护 EndpointSlice；不会自动选择所有 Pod。
              </p>
            )}
          </section>
          <ServicePorts
            ports={value.ports ?? []}
            type={value.type}
            onChange={(ports) => set({ ports })}
          />
          <div className="form-section">
            <Checkbox
              checked={value.publishNotReadyAddresses}
              onChange={(event) => set({ publishNotReadyAddresses: event.target.checked })}
            >
              发布尚未就绪的后端地址
            </Checkbox>
          </div>
          {value.type === 'LoadBalancer' && (
            <Alert
              showIcon
              type="info"
              title="外部地址分配依赖集群的负载均衡实现；提交成功后可能仍需等待。"
            />
          )}
        </>
      )}
    </>
  );
}
