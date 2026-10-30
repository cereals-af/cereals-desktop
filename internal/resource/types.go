// Package resource defines the cluster-independent resource browsing contract.
package resource

import "context"

// Type describes a discoverable Kubernetes resource, including custom resources.
type Type struct {
	Group      string   `json:"group"`
	Version    string   `json:"version"`
	Resource   string   `json:"resource"`
	Kind       string   `json:"kind"`
	Namespaced bool     `json:"namespaced"`
	Verbs      []string `json:"verbs"`
}

type Catalog struct {
	Resources []Type   `json:"resources"`
	Warnings  []string `json:"warnings"`
}

type Query struct {
	ClusterID     string `json:"clusterID"`
	Group         string `json:"group"`
	Version       string `json:"version"`
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Continue      string `json:"continue"`
}

type Ref struct {
	ClusterID string `json:"clusterID"`
	Group     string `json:"group"`
	Version   string `json:"version"`
	Resource  string `json:"resource"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type NodeSummary struct {
	Roles          string `json:"roles"`
	InternalIP     string `json:"internalIP"`
	KubeletVersion string `json:"kubeletVersion"`
	CPU            string `json:"cpu"`
	Memory         string `json:"memory"`
	Unschedulable  bool   `json:"unschedulable"`
}

type Item struct {
	Node      *NodeSummary      `json:"node,omitempty"`
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	CreatedAt string            `json:"createdAt"`
	Status    string            `json:"status"`
	Detail    string            `json:"detail"`
	Labels    map[string]string `json:"labels"`
}

type Page struct {
	Items           []Item `json:"items"`
	Continue        string `json:"continue"`
	ResourceVersion string `json:"resourceVersion"`
}

// Client owns one authenticated cluster connection. Implementations must honor
// cancellation and must never expose credentials through returned values.
type Client interface {
	Version(context.Context) (string, error)
	Discover(context.Context) (Catalog, error)
	List(context.Context, Query) (Page, error)
	YAML(context.Context, Ref) (string, error)
	Close()
}

// Resolver keeps request targets explicit instead of using a global current cluster.
type Resolver interface {
	Client(string) (Client, error)
}
