// Package workload adapts common workload forms to the shared manifest write pipeline.
package workload

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type Previewer interface {
	Preview(context.Context, manifest.Request) (manifest.Preview, error)
}

type Deployment struct {
	PodOptions *formvalue.Value `json:"podOptions"`
	Name       string           `json:"name"`
	Namespace  string           `json:"namespace"`
	Replicas   int32            `json:"replicas"`
	Containers []Container      `json:"containers"`
}

type Container struct {
	Advanced *formvalue.Value `json:"advanced"`
	// OriginalName identifies an existing container; empty adds a new container.
	OriginalName  string          `json:"originalName"`
	Name          string          `json:"name"`
	Image         string          `json:"image"`
	Command       []string        `json:"command"`
	Args          []string        `json:"args"`
	Ports         []ContainerPort `json:"ports"`
	CPURequest    string          `json:"cpuRequest"`
	CPULimit      string          `json:"cpuLimit"`
	MemoryRequest string          `json:"memoryRequest"`
	MemoryLimit   string          `json:"memoryLimit"`
	Environment   []Environment   `json:"environment"`
	Readiness     Probe           `json:"readiness"`
	Liveness      Probe           `json:"liveness"`
	Startup       Probe           `json:"startup"`
}

type ContainerPort struct {
	// OriginalKey identifies the original name (or unnamed port/protocol); empty adds a port.
	OriginalKey   string `json:"originalKey"`
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	ContainerPort int32  `json:"containerPort"`
	HostPort      int32  `json:"hostPort"`
	HostIP        string `json:"hostIP"`
}

type Environment struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Source is descriptive only. Referenced values remain on the server and cannot be changed by this form.
	Source string `json:"source"`
}

type Probe struct {
	Kind             string   `json:"kind"` // none, http, tcp, exec, keep (unsupported handler)
	Path             string   `json:"path"`
	Port             string   `json:"port"`
	Scheme           string   `json:"scheme"`
	Command          []string `json:"command"`
	InitialDelay     int32    `json:"initialDelay"`
	Timeout          int32    `json:"timeout"`
	Period           int32    `json:"period"`
	SuccessThreshold int32    `json:"successThreshold"`
	FailureThreshold int32    `json:"failureThreshold"`
}

type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type Status struct {
	State      string      `json:"state,omitempty"`
	Detail     string      `json:"detail,omitempty"`
	Phase      string      `json:"phase"`
	Strategy   string      `json:"strategy"`
	Paused     bool        `json:"paused"`
	Observed   bool        `json:"observed"`
	Replicas   int32       `json:"replicas"`
	Updated    int32       `json:"updated"`
	Ready      int32       `json:"ready"`
	Available  int32       `json:"available"`
	Conditions []Condition `json:"conditions"`
}

type Snapshot struct {
	Form     Deployment        `json:"form"`
	Baseline manifest.Baseline `json:"baseline"`
	Status   Status            `json:"status"`
}

type Request struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Action   string            `json:"action"` // create, update, scale (legacy), restart, pause, resume
	Form     Deployment        `json:"form"`
}
