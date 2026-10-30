package workloadstate

import (
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Observer supplements controller reports with UID-owned child Pods and Jobs.
// On restricted or incomplete reads it keeps the reported state and explains
// the missing verification in the detail rather than claiming a complete view.
type Observer interface {
	WorkloadState(context.Context, unstructured.Unstructured) (string, string)
}
