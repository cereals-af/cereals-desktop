// Package storage provides storage inspection and reviewable PVC mutations.
package storage

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Reader interface {
	GetObject(context.Context, resource.Ref) (*unstructured.Unstructured, error)
}

type Resolver interface{ StorageReader(string) (Reader, error) }

type Previewer interface {
	PreviewGuarded(context.Context, manifest.Request, []manifest.Dependency) (manifest.Preview, error)
}

type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type Claim struct {
	Baseline     manifest.Baseline `json:"baseline"`
	Phase        string            `json:"phase"`
	Requested    string            `json:"requested"`
	Capacity     string            `json:"capacity"`
	Allocated    string            `json:"allocated"`
	ResizeStatus string            `json:"resizeStatus"`
	AccessModes  []string          `json:"accessModes"`
	VolumeMode   string            `json:"volumeMode"`
	VolumeName   string            `json:"volumeName"`
	ClassName    string            `json:"className"`
	ClassMode    string            `json:"classMode"`
	Conditions   []Condition       `json:"conditions"`
	Deleting     bool              `json:"deleting"`
}

type Volume struct {
	Phase          string   `json:"phase"`
	Capacity       string   `json:"capacity"`
	AccessModes    []string `json:"accessModes"`
	VolumeMode     string   `json:"volumeMode"`
	ClassName      string   `json:"className"`
	ClaimName      string   `json:"claimName"`
	ClaimNamespace string   `json:"claimNamespace"`
	ClaimUID       string   `json:"claimUID"`
	ReclaimPolicy  string   `json:"reclaimPolicy"`
	Driver         string   `json:"driver"`
	MountOptions   []string `json:"mountOptions"`
	NodeAffinity   string   `json:"nodeAffinity"`
	Reason         string   `json:"reason"`
	Message        string   `json:"message"`
}

type Class struct {
	Provisioner       string   `json:"provisioner"`
	ReclaimPolicy     string   `json:"reclaimPolicy"`
	BindingMode       string   `json:"bindingMode"`
	AllowExpansion    bool     `json:"allowExpansion"`
	Default           bool     `json:"default"`
	MountOptions      []string `json:"mountOptions"`
	AllowedTopologies string   `json:"allowedTopologies"`
}

type ClaimForm struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Capacity    string   `json:"capacity"`
	AccessModes []string `json:"accessModes"`
	VolumeMode  string   `json:"volumeMode"`
	// ClassMode distinguishes an omitted class (default) from an explicit empty class (none).
	ClassMode string `json:"classMode"`
	ClassName string `json:"className"`
}

type CreateRequest struct {
	Target resource.Ref `json:"target"`
	Form   ClaimForm    `json:"form"`
}

type ExpandRequest struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Capacity string            `json:"capacity"`
}
