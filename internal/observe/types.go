// Package observe shares bounded change notifications across resource views.
package observe

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type Update struct {
	Revision uint64 `json:"revision"`
	Changes  uint64 `json:"changes"`
	State    string `json:"state"`
	Message  string `json:"message"`
}

// Change carries no resource payload, so Secrets cannot enter event buffers.
type Change struct {
	State, Message string
	Dirty          bool
}

type Client interface {
	ConnectionID() string
	Observe(context.Context, resource.Query, string, func(Change)) error
}

type Resolver interface{ WatchClient(string) (Client, error) }
