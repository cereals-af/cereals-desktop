// Package relations builds bounded, read-only resource relationships.
package relations

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Client interface {
	GetObject(context.Context, resource.Ref) (*unstructured.Unstructured, error)
	ListObjects(context.Context, resource.Ref, metav1.ListOptions) (*unstructured.UnstructuredList, error)
}

type Resolver interface{ RelationClient(string) (Client, error) }

type PodSummary struct {
	Ready      string   `json:"ready"`
	Restarts   int64    `json:"restarts"`
	Containers []string `json:"containers"`
}

type Node struct {
	Pod      *PodSummary  `json:"pod,omitempty"`
	Target   resource.Ref `json:"target"`
	UID      string       `json:"uid"`
	Kind     string       `json:"kind"`
	Status   string       `json:"status"`
	Detail   string       `json:"detail"`
	Revision string       `json:"revision"`
	Relation string       `json:"relation"`
}

type Event struct {
	UID     string `json:"uid"`
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Count   int64  `json:"count"`
	Time    string `json:"time"`
}

type Section struct {
	Key       string  `json:"key"`
	Title     string  `json:"title"`
	Items     []Node  `json:"items"`
	Events    []Event `json:"events"`
	Note      string  `json:"note"`
	Error     string  `json:"error"`
	Truncated bool    `json:"truncated"`
}

type Report struct {
	Root      Node      `json:"root"`
	Sections  []Section `json:"sections"`
	CheckedAt string    `json:"checkedAt"`
}
