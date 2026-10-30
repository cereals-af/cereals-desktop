import type { SavedFile } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/desktop/models';
import type {
  Selection as TransferSelection,
  Request as TransferRequest,
  Progress as TransferProgress,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/filetransfer/models';
export type { SavedFile, TransferSelection, TransferRequest, TransferProgress };
import type {
  Snapshot as NodeSnapshot,
  Pod as NodePod,
  PodPage as NodePodPage,
  CordonRequest,
  DrainRequest,
  Plan as DrainPlan,
  Progress as DrainProgress,
  DrainOptions,
  PodProgress,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/node/models';
export type {
  NodeSnapshot,
  NodePod,
  NodePodPage,
  CordonRequest,
  DrainRequest,
  DrainPlan,
  DrainProgress,
  DrainOptions,
  PodProgress,
};
export type {
  Profile,
  State,
  ConnectRequest,
  Connection,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/cluster/models';
export type {
  Type as ResourceType,
  Query,
  Ref,
  Item,
  Page,
  Catalog,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/resource/models';

import type {
  State,
  ConnectRequest,
  Connection,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/cluster/models';
import type {
  Query,
  Ref,
  Page,
  Catalog,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/resource/models';

export type {
  Baseline,
  Snapshot,
  Request as ManifestRequest,
  DeleteRequest,
  Preview,
  Result as MutationResult,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/manifest/models';
export type { Update as WatchUpdate } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/observe/models';
import type {
  Snapshot,
  Request,
  DeleteRequest,
  Preview,
  Result,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/manifest/models';
import type { Update } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/observe/models';
export type {
  Deployment,
  Container as WorkloadContainer,
  ContainerPort,
  Environment,
  Probe,
  Snapshot as WorkloadSnapshot,
  Status as WorkloadStatus,
  Request as WorkloadRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/workload/models';
import type {
  Snapshot as WorkloadSnapshot,
  Request as WorkloadRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/workload/models';

import type { Snapshot as ClusterOverview } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/overview/models';
export type { ClusterOverview };

import type {
  Query as UsageQuery,
  Snapshot as UsageSnapshot,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/metrics/models';
export type { UsageQuery, UsageSnapshot };
export type {
  Target as UsageTarget,
  Sample as UsageSample,
  Usage,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/metrics/models';

export interface DesktopAPI {
  resourceUsage(query: UsageQuery, signal?: AbortSignal): Promise<UsageSnapshot>;
  setDefaultCluster(clusterID: string): Promise<State>;
  clusterOverview(clusterID: string, signal?: AbortSignal): Promise<ClusterOverview>;
  visualSnapshot(target: Ref, reveal: boolean, signal?: AbortSignal): Promise<VisualSnapshot>;
  previewVisual(request: VisualRequest, signal?: AbortSignal): Promise<Preview>;
  previewRun(request: RunRequest, signal?: AbortSignal): Promise<Preview>;

  controller(target: Ref, signal?: AbortSignal): Promise<ControllerSnapshot>;
  previewController(request: ControllerRequest, signal?: AbortSignal): Promise<Preview>;
  controllerHistory(target: Ref, signal?: AbortSignal): Promise<DeploymentHistory>;
  previewControllerRollback(
    request: ControllerRollbackRequest,
    signal?: AbortSignal,
  ): Promise<Preview>;
  claim(target: Ref, signal?: AbortSignal): Promise<StorageClaim>;
  volume(target: Ref, signal?: AbortSignal): Promise<StorageVolume>;
  storageClass(target: Ref, signal?: AbortSignal): Promise<StorageClass>;
  previewClaim(request: ClaimCreateRequest, signal?: AbortSignal): Promise<Preview>;
  previewExpansion(request: ClaimExpandRequest, signal?: AbortSignal): Promise<Preview>;
  node(target: Ref, signal?: AbortSignal): Promise<NodeSnapshot>;
  nodePods(
    target: Ref,
    namespace: string,
    cursor: string,
    signal?: AbortSignal,
  ): Promise<NodePodPage>;
  previewCordon(request: CordonRequest, signal?: AbortSignal): Promise<Preview>;
  previewDrain(request: DrainRequest, signal?: AbortSignal): Promise<DrainPlan>;
  discardDrain(token: string): Promise<void>;
  startDrain(token: string, confirmation: string): Promise<string>;
  pollDrain(id: string, signal?: AbortSignal): Promise<DrainProgress>;
  cancelDrain(id: string): Promise<void>;
  closeDrain(id: string): Promise<void>;

  customDefinition(target: Ref, signal?: AbortSignal): Promise<CustomDefinition>;
  customList(query: Query, signal?: AbortSignal): Promise<CustomPage>;
  customSnapshot(target: Ref, signal?: AbortSignal): Promise<CustomSnapshot>;
  convertCustom(
    value: CustomValue | null,
    yaml: string,
    signal?: AbortSignal,
  ): Promise<CustomDraft>;
  previewCustom(request: CustomRequest, signal?: AbortSignal): Promise<Preview>;

  scale(target: Ref, signal?: AbortSignal): Promise<ScaleSnapshot>;
  previewScale(request: ScaleRequest, signal?: AbortSignal): Promise<Preview>;
  deploymentHistory(target: Ref, signal?: AbortSignal): Promise<DeploymentHistory>;
  previewRollback(request: RollbackRequest, signal?: AbortSignal): Promise<Preview>;
  workloadPermissions(target: Ref, signal?: AbortSignal): Promise<WorkloadAccess[]>;
  pod(target: Ref, signal?: AbortSignal): Promise<PodInfo>;
  startSession(request: SessionRequest): Promise<string>;
  pollSession(id: string, cursor: number, signal?: AbortSignal): Promise<SessionUpdate>;
  sessionInput(id: string, data: string): Promise<void>;
  resizeSession(id: string, size: TerminalSize): Promise<void>;
  stopSession(id: string): Promise<void>;
  closeSession(id: string): Promise<void>;
  touchSession(id: string): Promise<void>;
  exportLogs(id: string): Promise<SavedFile>;
  selectUpload(sessionID: string): Promise<TransferSelection>;
  releaseUpload(id: string): Promise<void>;
  startTransfer(request: TransferRequest): Promise<string>;
  pollTransfer(id: string): Promise<TransferProgress>;
  cancelTransfer(id: string): Promise<void>;
  closeTransfer(id: string): Promise<void>;
  schema(target: Ref, signal?: AbortSignal): Promise<SchemaDocument>;
  relations(target: Ref, signal?: AbortSignal): Promise<RelationReport>;
  state(): Promise<State>;
  importFile(): Promise<State>;
  importDefault(): Promise<State>;
  connect(request: ConnectRequest, signal?: AbortSignal): Promise<Connection>;
  disconnect(clusterID: string): Promise<void>;
  discover(clusterID: string, signal?: AbortSignal): Promise<Catalog>;
  list(query: Query, signal?: AbortSignal): Promise<Page>;
  yaml(ref: Ref, signal?: AbortSignal): Promise<string>;
  snapshot(ref: Ref, revealSecret: boolean, signal?: AbortSignal): Promise<Snapshot>;
  preview(request: Request, signal?: AbortSignal): Promise<Preview>;
  previewDelete(request: DeleteRequest, signal?: AbortSignal): Promise<Preview>;
  commit(token: string, signal?: AbortSignal): Promise<Result>;
  discard(token: string): Promise<void>;
  subscribe(query: Query, resourceVersion: string): Promise<string>;
  poll(subscription: string, revision: number, signal?: AbortSignal): Promise<Update>;
  unsubscribe(subscription: string): Promise<void>;
  deployment(target: Ref, signal?: AbortSignal): Promise<WorkloadSnapshot>;
  previewWorkload(request: WorkloadRequest, signal?: AbortSignal): Promise<Preview>;
  service(target: Ref, signal?: AbortSignal): Promise<ServiceSnapshot>;
  previewService(request: ServiceRequest, signal?: AbortSignal): Promise<Preview>;
  configMap(target: Ref, signal?: AbortSignal): Promise<ConfigMapSnapshot>;
  previewConfigMap(request: ConfigMapRequest, signal?: AbortSignal): Promise<Preview>;
}

import type {
  Claim as StorageClaim,
  Volume as StorageVolume,
  Class as StorageClass,
  ClaimForm,
  CreateRequest as ClaimCreateRequest,
  ExpandRequest as ClaimExpandRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/storage/models';
export type {
  StorageClaim,
  StorageVolume,
  StorageClass,
  ClaimForm,
  ClaimCreateRequest,
  ClaimExpandRequest,
};

export type { Document as SchemaDocument } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/schema/models';
export type {
  Report as RelationReport,
  Node as RelatedNode,
  Section as RelationSection,
  Event as ResourceEvent,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/relations/models';
import type { Document as SchemaDocument } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/schema/models';
import type { Report as RelationReport } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/relations/models';

export type {
  ServiceForm,
  Port as ServicePort,
  Selector as ServiceSelector,
  Snapshot as ServiceSnapshot,
  Request as ServiceRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/network/models';
export type {
  ConfigMap as ConfigMapForm,
  Entry as ConfigEntry,
  Snapshot as ConfigMapSnapshot,
  Request as ConfigMapRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/configuration/models';
import type {
  Snapshot as ServiceSnapshot,
  Request as ServiceRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/network/models';
import type {
  Snapshot as ConfigMapSnapshot,
  Request as ConfigMapRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/configuration/models';

export type {
  Pod as PodInfo,
  Request as SessionRequest,
  Update as SessionUpdate,
  Size as TerminalSize,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/session/models';
import type {
  Pod as PodInfo,
  Request as SessionRequest,
  Update as SessionUpdate,
  Size as TerminalSize,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/session/models';

export type {
  ScaleSnapshot,
  ScaleRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/manifest/models';
import type {
  ScaleSnapshot,
  ScaleRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/manifest/models';
export type {
  History as DeploymentHistory,
  Revision as DeploymentRevision,
  RollbackRequest,
  Access as WorkloadAccess,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/workload/models';
import type {
  History as DeploymentHistory,
  RollbackRequest,
  Access as WorkloadAccess,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/workload/models';

export type {
  Definition as CustomDefinition,
  Version as CustomVersion,
  Column as CustomColumn,
  Row as CustomRow,
  Page as CustomPage,
  Value as CustomValue,
  Field as CustomField,
  Draft as CustomDraft,
  Snapshot as CustomSnapshot,
  Request as CustomRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/customresource/models';
import type {
  Definition as CustomDefinition,
  Page as CustomPage,
  Value as CustomValue,
  Draft as CustomDraft,
  Snapshot as CustomSnapshot,
  Request as CustomRequest,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/customresource/models';

import type {
  ControllerForm,
  ControllerSnapshot,
  ControllerRequest,
  ControllerRollbackRequest,
  ClaimTemplate,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/workload/models';
export type {
  ControllerForm,
  ControllerSnapshot,
  ControllerRequest,
  ControllerRollbackRequest,
  ClaimTemplate,
};

import type {
  Snapshot as VisualSnapshot,
  Request as VisualRequest,
  RunRequest,
  SecretEntry,
} from '../../bindings/github.com/cereals-af/cereals-desktop/internal/resourceform/models';
export type { VisualSnapshot, VisualRequest, RunRequest, SecretEntry };
