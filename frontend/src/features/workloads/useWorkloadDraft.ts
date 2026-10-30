import { useAPI } from '../../api/context';
import type { Ref, WorkloadSnapshot } from '../../api/types';
import { useFormDraft } from '../resources/useFormDraft';
import type { WorkloadAction } from './actions';

export function useWorkloadDraft(
  target: Ref,
  snapshot: WorkloadSnapshot,
  action: WorkloadAction,
  onSuccess: () => void,
) {
  const api = useAPI();
  return useFormDraft({
    form: snapshot.form,
    baseline: snapshot.baseline,
    onSuccess,
    prepare: (form, baseline, signal) =>
      api.previewWorkload(
        {
          target: action === 'create' ? { ...target, namespace: form.namespace } : target,
          baseline,
          action,
          form,
        },
        signal,
      ),
    readBaseline: async (signal) => (await api.deployment(target, signal)).baseline,
  });
}
