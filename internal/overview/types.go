// Package overview provides read-only cluster capacity and resource counts.
package overview

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type ResourceCount struct {
	Type  resource.Type `json:"type"`
	Count *int64        `json:"count"`
	Error string        `json:"error"`
}

type Nodes struct {
	Count         int64             `json:"count"`
	Ready         int64             `json:"ready"`
	Unschedulable int64             `json:"unschedulable"`
	Capacity      map[string]string `json:"capacity"`
	Allocatable   map[string]string `json:"allocatable"`
}

type Snapshot struct {
	ClusterID string          `json:"clusterID"`
	UpdatedAt string          `json:"updatedAt"`
	Nodes     *Nodes          `json:"nodes"`
	NodeError string          `json:"nodeError"`
	Resources []ResourceCount `json:"resources"`
	Warnings  []string        `json:"warnings"`
}

// Client counts metadata only; resource bodies and credentials never leave this boundary.
type Client interface {
	Discover(context.Context) (resource.Catalog, error)
	CountResources(context.Context, resource.Type) (int64, error)
	OverviewNodes(context.Context) (Nodes, error)
}

type Resolver interface {
	OverviewClient(string) (Client, error)
}
