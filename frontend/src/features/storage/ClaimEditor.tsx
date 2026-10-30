import { Alert, Input } from 'antd';
import { useAPI } from '../../api/context';
import type { ClaimForm, Ref, StorageClaim } from '../../api/types';
import { FormSession } from '../resources/FormSession';
import { ClaimFields } from './ClaimFields';

interface Props {
  target: Ref;
  namespace: string;
  onSuccess: () => void;
  onDone: (ref: Ref) => void;
  onBack: () => void;
}

export function ClaimCreator(props: Props) {
  const api = useAPI();
  const initial: ClaimForm = {
    name: '',
    namespace: props.namespace,
    capacity: '10Gi',
    accessModes: ['ReadWriteOnce'],
    volumeMode: 'Filesystem',
    classMode: 'default',
    className: '',
  };
  return (
    <FormSession
      {...props}
      creating
      initial={initial}
      baseline={{ uid: '', resourceVersion: '' }}
      readBaseline={() => Promise.resolve({ uid: '', resourceVersion: '' })}
      prepare={(form, _baseline, signal) =>
        api.previewClaim({ target: { ...props.target, namespace: form.namespace }, form }, signal)
      }
    >
      {(form, setForm) => (
        <ClaimFields value={form} onChange={setForm} clusterID={props.target.clusterID} />
      )}
    </FormSession>
  );
}

export function ClaimExpander(props: Props & { claim: StorageClaim }) {
  const api = useAPI();
  return (
    <FormSession
      {...props}
      creating={false}
      initial={props.claim.requested}
      baseline={props.claim.baseline}
      readBaseline={async (signal) => (await api.claim(props.target, signal)).baseline}
      prepare={(capacity, baseline, signal) =>
        api.previewExpansion({ target: props.target, baseline, capacity }, signal)
      }
    >
      {(capacity, setCapacity) => (
        <div className="storage-fields">
          <Alert
            showIcon
            type="warning"
            title="扩容申请不能用于缩容"
            description={`打开时申请容量 ${props.claim.requested}，已报告容量 ${props.claim.capacity || '未知'}。预检会重新校验 PVC 和存储类；提交后请查看 Conditions 与事件，确认控制器及文件系统处理结果。`}
          />
          <label>
            新的申请容量
            <Input
              aria-label="新的申请容量"
              value={capacity}
              onChange={(event) => setCapacity(event.target.value)}
              placeholder="例如 20Gi"
            />
          </label>
        </div>
      )}
    </FormSession>
  );
}
