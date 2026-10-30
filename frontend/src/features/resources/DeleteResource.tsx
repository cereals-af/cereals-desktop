import { Alert, Button, Checkbox, Drawer, Input, InputNumber, Select, Space, Spin } from 'antd';
import type { Profile, Ref } from '../../api/types';
import { errorMessage } from '../../lib/errors';
import { MutationReview } from './MutationReview';
import { useDeleteResource } from './useDeleteResource';

export function DeleteResource({
  target,
  profile,
  onClose,
  onSuccess,
}: {
  target: Ref;
  profile: Profile;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const {
    policy,
    setPolicy,
    useGrace,
    setUseGrace,
    grace,
    setGrace,
    name,
    setName,
    error,
    phase,
    snapshot,
    preview,
    commit,
    back,
  } = useDeleteResource(target, onSuccess);
  return (
    <Drawer
      open
      size={1080}
      className="manifest-drawer"
      title={`删除资源 · ${profile.context}`}
      onClose={onClose}
      destroyOnHidden
    >
      <p className="muted">
        {profile.server} · {target.namespace || '集群作用域'} / {target.name}
      </p>
      <Alert
        type="warning"
        showIcon
        title="删除可能影响依赖资源和业务运行"
        description="Namespace 删除影响整个命名空间；CRD 删除影响该类型的全部实例。预检不会执行实际删除，也不会枚举全部依赖；直接删除 Pod 不受 PDB 保护；finalizer 与控制器可能影响完成时间。"
      />
      {(error || snapshot.isError) && (
        <Alert type="error" showIcon title={error || errorMessage(snapshot.error)} />
      )}
      {phase.kind === 'done' ? (
        <Alert type="success" title="删除请求已接受" description={phase.message} />
      ) : phase.kind === 'review' ? (
        <>
          <MutationReview preview={phase.preview} />
          <div className="mutation-actions">
            <Input
              aria-label="确认删除资源名称"
              placeholder={`输入 ${target.name} 确认`}
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
            <Button onClick={back}>返回</Button>
            <Button
              danger
              type="primary"
              disabled={name !== target.name}
              onClick={() => void commit()}
            >
              确认删除
            </Button>
          </div>
        </>
      ) : (
        <Space orientation="vertical" size="large" className="delete-options">
          <Select
            aria-label="级联删除策略"
            value={policy}
            disabled={phase.kind === 'busy'}
            onChange={setPolicy}
            options={[
              { value: 'Background', label: '后台级联：先接受删除，再由控制器清理依赖' },
              { value: 'Foreground', label: '前台级联：等待依赖清理后完成删除' },
              { value: 'Orphan', label: '保留依赖：解除从属关系，保留依赖对象' },
            ]}
          />
          <Space>
            <Checkbox
              checked={useGrace}
              disabled={phase.kind === 'busy'}
              onChange={(event) => setUseGrace(event.target.checked)}
            >
              指定宽限期
            </Checkbox>
            <InputNumber
              aria-label="删除宽限期秒数"
              min={0}
              max={86400}
              disabled={!useGrace || phase.kind === 'busy'}
              value={grace}
              onChange={(value) => setGrace(value ?? 30)}
            />
            秒
          </Space>
          {snapshot.isPending ? (
            <Spin />
          ) : (
            <Button
              danger
              disabled={!snapshot.data}
              loading={phase.kind === 'busy'}
              onClick={() => void preview()}
            >
              预检删除
            </Button>
          )}
        </Space>
      )}
    </Drawer>
  );
}
