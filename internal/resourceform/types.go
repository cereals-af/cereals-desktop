// Package resourceform adapts built-in resource forms to the guarded manifest pipeline.
package resourceform

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type Snapshot struct {
	State    string            `json:"state,omitempty"`
	Detail   string            `json:"detail,omitempty"`
	Baseline manifest.Baseline `json:"baseline"`
	Value    formvalue.Value   `json:"value"`
	Status   formvalue.Value   `json:"status"`
	Entries  []SecretEntry     `json:"entries"`
	Revealed bool              `json:"revealed"`
}
type SecretEntry struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Encoding string `json:"encoding"` // text or base64 for binary data
}
type Request struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Mode     string            `json:"mode"`
	Value    formvalue.Value   `json:"value"`
	Entries  []SecretEntry     `json:"entries"`
	Revealed bool              `json:"revealed"`
}
type RunRequest struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Name     string            `json:"name"`
}
type writer interface {
	Preview(context.Context, manifest.Request) (manifest.Preview, error)
	PreviewGuarded(context.Context, manifest.Request, []manifest.Dependency) (manifest.Preview, error)
}
