// Package node owns node inspection and bounded, reviewable maintenance tasks.
package node

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Updated string `json:"updated"`
}
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}
type Snapshot struct {
	Baseline      manifest.Baseline `json:"baseline"`
	Unschedulable bool              `json:"unschedulable"`
	Roles         []string          `json:"roles"`
	Addresses     map[string]string `json:"addresses"`
	Info          map[string]string `json:"info"`
	Capacity      map[string]string `json:"capacity"`
	Allocatable   map[string]string `json:"allocatable"`
	Labels        map[string]string `json:"labels"`
	Taints        []Taint           `json:"taints"`
	Conditions    []Condition       `json:"conditions"`
}
type Pod struct {
	UID         string   `json:"uid"`
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Phase       string   `json:"phase"`
	Owner       string   `json:"owner"`
	Category    string   `json:"category"`
	LocalData   []string `json:"localData"`
	Terminating bool     `json:"terminating"`
}
type PodPage struct {
	Pods     []Pod  `json:"pods"`
	Continue string `json:"continue"`
}
type CordonRequest struct {
	Target        resource.Ref      `json:"target"`
	Baseline      manifest.Baseline `json:"baseline"`
	Unschedulable bool              `json:"unschedulable"`
}
type DrainOptions struct {
	IgnoreDaemonSets bool  `json:"ignoreDaemonSets"`
	AllowUnmanaged   bool  `json:"allowUnmanaged"`
	AllowLocalData   bool  `json:"allowLocalData"`
	TimeoutSeconds   int32 `json:"timeoutSeconds"`
}
type DrainRequest struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Options  DrainOptions      `json:"options"`
}
type PodProgress struct {
	Pod     Pod    `json:"pod"`
	State   string `json:"state"`
	Message string `json:"message"`
}
type Plan struct {
	Token     string           `json:"token"`
	Cordon    manifest.Preview `json:"cordon"`
	Pods      []PodProgress    `json:"pods"`
	ExpiresAt string           `json:"expiresAt"`
}
type Progress struct {
	Target  resource.Ref  `json:"target"`
	State   string        `json:"state"`
	Message string        `json:"message"`
	Pods    []PodProgress `json:"pods"`
	Done    bool          `json:"done"`
}

// Reader deliberately separates inspection from eviction and mutation.
type Reader interface {
	GetObject(context.Context, resource.Ref) (*unstructured.Unstructured, error)
	ListObjects(context.Context, resource.Ref, metav1.ListOptions) (*unstructured.UnstructuredList, error)
}
type Eviction struct {
	Target   resource.Ref
	Baseline manifest.Baseline
	DryRun   bool
}
type EvictionResult struct{ Waiting bool }
type Client interface {
	Reader
	ConnectionID() string
	SessionDone() <-chan struct{}
	ReadDrainPod(context.Context, resource.Ref) (*unstructured.Unstructured, error)
	EvictPod(context.Context, Eviction) (EvictionResult, error)
}
type Resolver interface{ NodeClient(string) (Client, error) }
type Writer interface {
	Preview(context.Context, manifest.Request) (manifest.Preview, error)
	Commit(context.Context, string) (manifest.Result, error)
	Discard(string)
}
