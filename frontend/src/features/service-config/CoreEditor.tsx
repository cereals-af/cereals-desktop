import { useAPI } from '../../api/context';
import type { Ref } from '../../api/types';
import { FormSession } from '../resources/FormSession';
import { ConfigMapFields } from './ConfigMapFields';
import { ServiceFields } from './ServiceFields';
import type { FormDocument } from './models';

export function CoreEditor({
  document,
  target,
  onSuccess,
  onDone,
  onBack,
}: {
  document: FormDocument;
  target: Ref;
  onSuccess: () => void;
  onDone: (ref: Ref) => void;
  onBack: () => void;
}) {
  const api = useAPI();
  const creating = !target.name;
  const mode = creating ? 'create' : 'patch';
  const common = { creating, onSuccess, onDone, onBack, baseline: document.snapshot.baseline };
  if (document.kind === 'service')
    return (
      <FormSession
        {...common}
        initial={document.snapshot.form}
        prepare={(form, baseline, signal) =>
          api.previewService(
            {
              target: creating ? { ...target, namespace: form.namespace } : target,
              mode,
              baseline,
              form,
            },
            signal,
          )
        }
        readBaseline={async (signal) => (await api.service(target, signal)).baseline}
      >
        {(form, setForm) => (
          <ServiceFields
            value={form}
            original={document.snapshot.form}
            creating={creating}
            clusterID={target.clusterID}
            onChange={setForm}
          />
        )}
      </FormSession>
    );
  return (
    <FormSession
      {...common}
      initial={document.snapshot.form}
      prepare={(form, baseline, signal) =>
        api.previewConfigMap(
          {
            target: creating ? { ...target, namespace: form.namespace } : target,
            mode,
            baseline,
            form,
          },
          signal,
        )
      }
      readBaseline={async (signal) => (await api.configMap(target, signal)).baseline}
    >
      {(form, setForm) => (
        <ConfigMapFields
          value={form}
          originalImmutable={document.snapshot.form.immutable}
          creating={creating}
          clusterID={target.clusterID}
          onChange={setForm}
        />
      )}
    </FormSession>
  );
}
