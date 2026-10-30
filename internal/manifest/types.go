// Package manifest owns validated, reviewable resource mutations.
package manifest

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Baseline struct {
	UID             string `json:"uid"`
	ResourceVersion string `json:"resourceVersion"`
}

type Snapshot struct {
	YAML     string   `json:"yaml"`
	Baseline Baseline `json:"baseline"`
	Editable bool     `json:"editable"`
}

type Request struct {
	Target       resource.Ref `json:"target"`
	Mode         string       `json:"mode"`
	YAML         string       `json:"yaml"`
	Baseline     Baseline     `json:"baseline"`
	RevealSecret bool         `json:"revealSecret"`
}

type DeleteRequest struct {
	Target       resource.Ref `json:"target"`
	Baseline     Baseline     `json:"baseline"`
	Propagation  string       `json:"propagation"`
	UseGrace     bool         `json:"useGrace"`
	GraceSeconds int64        `json:"graceSeconds"`
}

type Preview struct {
	Token     string       `json:"token"`
	Target    resource.Ref `json:"target"`
	Mode      string       `json:"mode"`
	Before    string       `json:"before"`
	Desired   string       `json:"desired"`
	After     string       `json:"after"`
	ExpiresAt string       `json:"expiresAt"`
}

type Result struct {
	Target  resource.Ref `json:"target"`
	Message string       `json:"message"`
}

// Operation is immutable once its server dry run has been reviewed.
type Operation struct {
	Subresource string
	Target      resource.Ref
	Mode        string
	Data        []byte
}

// Client is a mutation boundary; Kubernetes types never cross the Wails boundary.
type Client interface {
	ConnectionID() string
	Describe(context.Context, resource.Ref) (resource.Type, error)
	GetObject(context.Context, resource.Ref) (*unstructured.Unstructured, error)
	WriteObject(context.Context, Operation, bool) (*unstructured.Unstructured, error)
	DeleteObject(context.Context, DeleteRequest, bool) error
}

type Resolver interface{ ManifestClient(string) (Client, error) }
