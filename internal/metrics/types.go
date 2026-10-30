// Package metrics exposes bounded, read-only resource usage snapshots.
package metrics

import "context"

type Target struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

type Query struct {
	ClusterID string   `json:"clusterID"`
	Resource  string   `json:"resource"`
	Namespace string   `json:"namespace"`
	Targets   []Target `json:"targets"`
}

type Usage struct {
	CPUCores    float64 `json:"cpuCores"`
	MemoryBytes float64 `json:"memoryBytes"`
}

type Sample struct {
	Target    Target `json:"target"`
	State     string `json:"state"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Window    string `json:"window"`
	Usage     *Usage `json:"usage,omitempty"`
}

type Snapshot struct {
	State   string   `json:"state"`
	Message string   `json:"message"`
	Items   []Sample `json:"items"`
}

// Reader belongs to a single authenticated connection; cancellation includes
// disconnecting that connection, even when a replacement has the same profile.
type Reader interface {
	ResourceUsage(context.Context, Query) (Snapshot, error)
}

type Resolver interface{ MetricsReader(string) (Reader, error) }
