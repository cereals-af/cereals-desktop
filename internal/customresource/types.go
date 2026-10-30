// Package customresource provides CRD metadata and lossless schema-driven editing.
package customresource

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type Column struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	JSONPath    string `json:"jsonPath"`
	Priority    int64  `json:"priority"`
}
type Version struct {
	Name       string   `json:"name"`
	Served     bool     `json:"served"`
	Storage    bool     `json:"storage"`
	Deprecated bool     `json:"deprecated"`
	Warning    string   `json:"warning"`
	Columns    []Column `json:"columns"`
}
type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}
type Definition struct {
	Name           string            `json:"name"`
	Baseline       manifest.Baseline `json:"baseline"`
	Group          string            `json:"group"`
	Resource       string            `json:"resource"`
	Kind           string            `json:"kind"`
	Namespaced     bool              `json:"namespaced"`
	Versions       []Version         `json:"versions"`
	StoredVersions []string          `json:"storedVersions"`
	Conditions     []Condition       `json:"conditions"`
}

// Value never carries a JSON number across the JavaScript boundary. Object fields
// not rendered by the schema remain in this tree and survive the round trip.
type Value = formvalue.Value

type Field struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Nullable    bool     `json:"nullable"`
	Enum        []string `json:"enum"`
	Fields      []Field  `json:"fields"`
	Items       *Field   `json:"items"`
	Reason      string   `json:"reason"`
}
type Draft struct {
	Value Value  `json:"value"`
	YAML  string `json:"yaml"`
}
type Snapshot struct {
	Baseline   manifest.Baseline `json:"baseline"`
	Definition manifest.Baseline `json:"definition"`
	Fields     []Field           `json:"fields"`
	Draft      Draft             `json:"draft"`
}
type Request struct {
	Target     resource.Ref      `json:"target"`
	Baseline   manifest.Baseline `json:"baseline"`
	Definition manifest.Baseline `json:"definition"`
	Mode       string            `json:"mode"`
	Value      *Value            `json:"value"`
	YAML       string            `json:"yaml"`
}
type Row struct {
	UID       string   `json:"uid"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Cells     []string `json:"cells"`
}
type Page struct {
	Columns         []Column `json:"columns"`
	Rows            []Row    `json:"rows"`
	Continue        string   `json:"continue"`
	ResourceVersion string   `json:"resourceVersion"`
}
type tableReader interface {
	CustomTable(context.Context, resource.Query) (Page, error)
}
type Previewer interface {
	PreviewGuarded(context.Context, manifest.Request, []manifest.Dependency) (manifest.Preview, error)
}
