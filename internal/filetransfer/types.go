// Package filetransfer owns bounded, cancellable transfers for explicit Pod sessions.
package filetransfer

import (
	"context"
	"errors"
	"io"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/session"
)

const MaxFileSize int64 = 2 << 30

var ErrExists = errors.New("容器中已存在同名文件，请确认覆盖后重试")

type Selection struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}
type Request struct {
	SessionID   string `json:"sessionID"`
	Direction   string `json:"direction"`
	SelectionID string `json:"selectionID"`
	RemotePath  string `json:"remotePath"`
	Overwrite   bool   `json:"overwrite"`
}
type Progress struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Name       string `json:"name"`
	RemotePath string `json:"remotePath"`
	Bytes      int64  `json:"bytes"`
	Total      int64  `json:"total"`
	Message    string `json:"message"`
	Done       bool   `json:"done"`
}
type Sessions interface {
	FileTarget(string) (session.FileTarget, error)
}
type Dialogs interface {
	PickUpload() (string, error)
	SaveFile(string) (string, error)
}
type Client interface {
	ConnectionID() string
	SessionDone() <-chan struct{}
	Pod(context.Context, resource.Ref) (session.Pod, error)
	FileExec(context.Context, session.Request, []string, io.Reader, io.Writer) error
}
type Resolver interface{ FileClient(string) (Client, error) }
