// Package session owns bounded, leased Pod operation sessions.
package session

import (
	"context"
	"errors"
	"io"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

// ErrPodGone distinguishes a confirmed deletion from transient lookup failures.
var ErrPodGone = errors.New("Pod 已删除，请重新选择容器组")

type Container struct {
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	State    string  `json:"state"`
	Restarts int32   `json:"restarts"`
	Ports    []int32 `json:"ports"`
}
type Pod struct {
	UID        string      `json:"uid"`
	Phase      string      `json:"phase"`
	Node       string      `json:"node"`
	Containers []Container `json:"containers"`
}
type Request struct {
	Target       resource.Ref `json:"target"`
	UID          string       `json:"uid"`
	Kind         string       `json:"kind"`
	Container    string       `json:"container"`
	Follow       bool         `json:"follow"`
	Previous     bool         `json:"previous"`
	Timestamps   bool         `json:"timestamps"`
	TailLines    int64        `json:"tailLines"`
	SinceSeconds int64        `json:"sinceSeconds"`
	Command      []string     `json:"command"`
	Columns      uint16       `json:"columns"`
	Rows         uint16       `json:"rows"`
	LocalPort    uint16       `json:"localPort"`
	RemotePort   uint16       `json:"remotePort"`
}
type Size struct {
	Columns uint16 `json:"columns"`
	Rows    uint16 `json:"rows"`
}
type Update struct {
	// Cursor acknowledges bytes only after the renderer has consumed them.
	Cursor    uint64 `json:"cursor"`
	Data      string `json:"data"` // Base64: chunk boundaries may split UTF-8 characters.
	State     string `json:"state"`
	Message   string `json:"message"`
	LocalPort uint16 `json:"localPort"`
	ExitCode  int    `json:"exitCode"`
	Done      bool   `json:"done"`
}
type Streams struct {
	Input  io.Reader
	Output io.Writer
	Sizes  <-chan Size
	Ready  func(uint16)
}
type Outcome struct{ ExitCode int }
type Client interface {
	ConnectionID() string
	SessionDone() <-chan struct{}
	Pod(context.Context, resource.Ref) (Pod, error)
	RunSession(context.Context, Request, Streams) (Outcome, error)
}
type Resolver interface{ SessionClient(string) (Client, error) }
