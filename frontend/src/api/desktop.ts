import type { CancellablePromise } from '@wailsio/runtime';
import type { DesktopAPI } from './types';

async function cancellable<T>(call: CancellablePromise<T>, signal?: AbortSignal): Promise<T> {
  const cancel = () => {
    // Cancellation is best effort; the original call still reports its own result or error.
    void call.cancel().catch(() => undefined);
  };
  signal?.addEventListener('abort', cancel, { once: true });
  if (signal?.aborted) cancel();
  try {
    return await call;
  } finally {
    signal?.removeEventListener('abort', cancel);
  }
}

const workspace = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/desktop/workspaceservice');
const resources = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/resource/service');
const overview = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/overview/service');
const metrics = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/metrics/service');
const manifests = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/manifest/service');
const watches = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/observe/service');
const workloads = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/workload/service');
const network = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/network/service');
const configuration = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/configuration/service');
const schemas = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/schema/service');
const relations = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/relations/service');

const sessionFiles = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/desktop/sessionfilesservice');
const transfers = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/filetransfer/service');
const sessions = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/session/service');

const custom = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/customresource/service');

const nodes = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/node/service');

const storage = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/storage/service');

const visual = () =>
  import('../../bindings/github.com/cereals-af/cereals-desktop/internal/resourceform/service');
export const desktopAPI: DesktopAPI = {
  resourceUsage: async (query, signal) => cancellable((await metrics()).Snapshot(query), signal),
  setDefaultCluster: async (clusterID) => (await workspace()).SetDefault(clusterID),
  clusterOverview: async (clusterID, signal) =>
    cancellable((await overview()).Snapshot(clusterID), signal),
  visualSnapshot: async (target, reveal, signal) =>
    cancellable((await visual()).Snapshot(target, reveal), signal),
  previewVisual: async (request, signal) => cancellable((await visual()).Preview(request), signal),
  previewRun: async (request, signal) => cancellable((await visual()).PreviewRun(request), signal),

  controller: async (target, signal) => cancellable((await workloads()).Controller(target), signal),
  previewController: async (request, signal) =>
    cancellable((await workloads()).PreviewController(request), signal),
  controllerHistory: async (target, signal) =>
    cancellable((await workloads()).ControllerHistory(target), signal),
  previewControllerRollback: async (request, signal) =>
    cancellable((await workloads()).PreviewControllerRollback(request), signal),
  claim: async (target, signal) => cancellable((await storage()).Claim(target), signal),
  volume: async (target, signal) => cancellable((await storage()).Volume(target), signal),
  storageClass: async (target, signal) => cancellable((await storage()).Class(target), signal),
  previewClaim: async (request, signal) =>
    cancellable((await storage()).PreviewCreate(request), signal),
  previewExpansion: async (request, signal) =>
    cancellable((await storage()).PreviewExpand(request), signal),
  node: async (target, signal) => cancellable((await nodes()).Inspect(target), signal),
  nodePods: async (target, namespace, cursor, signal) =>
    cancellable((await nodes()).Pods(target, namespace, cursor), signal),
  previewCordon: async (request, signal) =>
    cancellable((await nodes()).PreviewCordon(request), signal),
  previewDrain: async (request, signal) =>
    cancellable((await nodes()).PreviewDrain(request), signal),
  discardDrain: async (token) => (await nodes()).DiscardDrain(token),
  startDrain: async (token, confirmation) => (await nodes()).StartDrain(token, confirmation),
  pollDrain: async (id, signal) => cancellable((await nodes()).PollDrain(id), signal),
  cancelDrain: async (id) => (await nodes()).CancelDrain(id),
  closeDrain: async (id) => (await nodes()).CloseDrain(id),

  customDefinition: async (target, signal) =>
    cancellable((await custom()).Definition(target), signal),
  customList: async (query, signal) => cancellable((await custom()).List(query), signal),
  customSnapshot: async (target, signal) => cancellable((await custom()).Snapshot(target), signal),
  convertCustom: async (value, yaml, signal) =>
    cancellable((await custom()).Convert(value, yaml), signal),
  previewCustom: async (request, signal) => cancellable((await custom()).Preview(request), signal),

  scale: async (target, signal) => cancellable((await manifests()).Scale(target), signal),
  previewScale: async (request, signal) =>
    cancellable((await manifests()).PreviewScale(request), signal),
  deploymentHistory: async (target, signal) =>
    cancellable((await workloads()).History(target), signal),
  previewRollback: async (request, signal) =>
    cancellable((await workloads()).PreviewRollback(request), signal),
  workloadPermissions: async (target, signal) =>
    (await cancellable((await workloads()).Permissions(target), signal)) ?? [],
  pod: async (target, signal) => cancellable((await sessions()).Pod(target), signal),
  startSession: async (request) => (await sessions()).Start(request),
  pollSession: async (id, cursor, signal) =>
    cancellable((await sessions()).Poll(id, cursor), signal),
  sessionInput: async (id, data) => (await sessions()).Input(id, data),
  resizeSession: async (id, size) => (await sessions()).Resize(id, size),
  stopSession: async (id) => (await sessions()).Stop(id),
  closeSession: async (id) => (await sessions()).Close(id),
  touchSession: async (id) => (await sessions()).Touch(id),
  exportLogs: async (id) => (await sessionFiles()).ExportLogs(id),
  selectUpload: async (id) => (await transfers()).SelectUpload(id),
  releaseUpload: async (id) => (await transfers()).ReleaseSelection(id),
  startTransfer: async (request) => (await transfers()).Start(request),
  pollTransfer: async (id) => (await transfers()).Poll(id),
  cancelTransfer: async (id) => (await transfers()).Cancel(id),
  closeTransfer: async (id) => (await transfers()).Close(id),
  schema: async (target, signal) => cancellable((await schemas()).Get(target), signal),
  relations: async (target, signal) => cancellable((await relations()).Inspect(target), signal),
  state: async () => (await workspace()).State(),
  importFile: async () => (await workspace()).ImportFile(),
  importDefault: async () => (await workspace()).ImportDefault(),
  connect: async (request, signal) => cancellable((await workspace()).Connect(request), signal),
  disconnect: async (id) => (await workspace()).Disconnect(id),
  discover: async (id, signal) => cancellable((await resources()).Discover(id), signal),
  list: async (query, signal) => cancellable((await resources()).List(query), signal),
  yaml: async (ref, signal) => cancellable((await resources()).YAML(ref), signal),
  snapshot: async (ref, reveal, signal) =>
    cancellable((await manifests()).Snapshot(ref, reveal), signal),
  preview: async (request, signal) => cancellable((await manifests()).Preview(request), signal),
  previewDelete: async (request, signal) =>
    cancellable((await manifests()).PreviewDelete(request), signal),
  commit: async (token, signal) => cancellable((await manifests()).Commit(token), signal),
  discard: async (token) => (await manifests()).Discard(token),
  subscribe: async (query, version) => (await watches()).Subscribe(query, version),
  poll: async (id, revision, signal) => cancellable((await watches()).Poll(id, revision), signal),
  unsubscribe: async (id) => (await watches()).Unsubscribe(id),
  deployment: async (target, signal) => cancellable((await workloads()).Deployment(target), signal),
  previewWorkload: async (request, signal) =>
    cancellable((await workloads()).Preview(request), signal),
  service: async (target, signal) => cancellable((await network()).Snapshot(target), signal),
  previewService: async (request, signal) =>
    cancellable((await network()).Preview(request), signal),
  configMap: async (target, signal) =>
    cancellable((await configuration()).ConfigMap(target), signal),
  previewConfigMap: async (request, signal) =>
    cancellable((await configuration()).Preview(request), signal),
};
