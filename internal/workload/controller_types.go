package workload

import (
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

// ControllerForm exposes only supported fields; edits are applied to the full server object.
type ControllerForm struct {
	Workload            Deployment      `json:"workload"`
	Strategy            string          `json:"strategy"`
	Partition           int32           `json:"partition"`
	MaxUnavailable      string          `json:"maxUnavailable"`
	MaxSurge            string          `json:"maxSurge"`
	ServiceName         string          `json:"serviceName"`
	PodManagementPolicy string          `json:"podManagementPolicy"`
	RetainDeleted       string          `json:"retainDeleted"`
	RetainScaled        string          `json:"retainScaled"`
	NodeSelector        []Label         `json:"nodeSelector"`
	Claims              []ClaimTemplate `json:"claims"`
}
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type ClaimTemplate struct {
	Name        string   `json:"name"`
	Capacity    string   `json:"capacity"`
	ClassMode   string   `json:"classMode"`
	ClassName   string   `json:"className"`
	AccessModes []string `json:"accessModes"`
	VolumeMode  string   `json:"volumeMode"`
	MountPath   string   `json:"mountPath"`
}
type ControllerStatus struct {
	State           string      `json:"state,omitempty"`
	Detail          string      `json:"detail,omitempty"`
	Phase           string      `json:"phase"`
	Observed        bool        `json:"observed"`
	Desired         int64       `json:"desired"`
	Current         int64       `json:"current"`
	Ready           int64       `json:"ready"`
	Updated         int64       `json:"updated"`
	Available       int64       `json:"available"`
	Unavailable     int64       `json:"unavailable"`
	Misscheduled    int64       `json:"misscheduled"`
	CurrentRevision string      `json:"currentRevision"`
	UpdateRevision  string      `json:"updateRevision"`
	Conditions      []Condition `json:"conditions"`
	Scheduling      string      `json:"scheduling"`
}
type ControllerSnapshot struct {
	Form     ControllerForm    `json:"form"`
	Baseline manifest.Baseline `json:"baseline"`
	Status   ControllerStatus  `json:"status"`
	Owner    *resource.Ref     `json:"owner"`
	OwnerUID string            `json:"ownerUID"`
}
type ControllerRequest struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Action   string            `json:"action"`
	Form     ControllerForm    `json:"form"`
}
type ControllerRollbackRequest struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Name     string            `json:"name"`
	Source   manifest.Baseline `json:"source"`
	Revision string            `json:"revision"`
}
