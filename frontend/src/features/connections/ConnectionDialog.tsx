import { useState } from 'react';
import { Alert, Checkbox, Modal } from 'antd';
import type { Profile } from '../../api/types';

interface Props {
  profile: Profile;
  onClose: () => void;
  onConfirm: (trust: boolean) => void;
  isDefault?: boolean;
}

export function ConnectionDialog({ profile, onClose, onConfirm, isDefault }: Props) {
  const [trusted, setTrusted] = useState(false);
  return (
    <Modal
      title="确认集群连接"
      open
      onCancel={onClose}
      onOk={() => onConfirm(trusted)}
      okText="连接集群"
      cancelText="取消"
      okButtonProps={{ disabled: Boolean(profile.execCommand) && !trusted }}
    >
      <div className="connection-details">
        <span>Context</span>
        <strong>{profile.context}</strong>
        <span>API Server</span>
        <code>{profile.server}</code>
        <span>命名空间</span>
        <strong>{profile.namespace}</strong>
      </div>
      {isDefault && (
        <p className="muted">连接成功后记住此配置的确认；配置未变化时，下次启动自动连接。</p>
      )}
      {profile.insecureTLS && <Alert type="warning" showIcon title="此配置关闭了 TLS 证书校验" />}
      {profile.execCommand && (
        <div className="trust-plugin">
          <Alert
            type="info"
            showIcon
            title="需要运行本地认证插件"
            description={<code>{profile.execCommand}</code>}
          />
          <Checkbox checked={trusted} onChange={(event) => setTrusted(event.target.checked)}>
            我信任此 kubeconfig，并允许其认证插件运行
          </Checkbox>
        </div>
      )}
    </Modal>
  );
}
