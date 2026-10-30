// Package configuration provides visual editing of non-secret configuration resources.
package configuration

import (
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ConfigMap struct {
	Name       string  `json:"name"`
	Namespace  string  `json:"namespace"`
	Data       []Entry `json:"data"`
	BinaryData []Entry `json:"binaryData"`
	Immutable  bool    `json:"immutable"`
}

type Snapshot struct {
	Form     ConfigMap         `json:"form"`
	Baseline manifest.Baseline `json:"baseline"`
}

type Request struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Mode     string            `json:"mode"`
	Form     ConfigMap         `json:"form"`
}
