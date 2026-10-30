// Package cluster imports kubeconfig snapshots and owns explicit cluster sessions.
package cluster

import (
	"context"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/client-go/rest"
)

type Profile struct {
	ID          string `json:"id"`
	Context     string `json:"context"`
	Cluster     string `json:"cluster"`
	Server      string `json:"server"`
	Namespace   string `json:"namespace"`
	Source      string `json:"source"`
	ExecCommand string `json:"execCommand"`
	InsecureTLS bool   `json:"insecureTLS"`
	Connected   bool   `json:"connected"`
	Version     string `json:"version"`
}

type State struct {
	DefaultClusterID         string    `json:"defaultClusterID"`
	DefaultNeedsConfirmation bool      `json:"defaultNeedsConfirmation"`
	Profiles                 []Profile `json:"profiles"`
	Warnings                 []string  `json:"warnings"`
}

type ConnectRequest struct {
	ClusterID string `json:"clusterID"`
	TrustExec bool   `json:"trustExec"`
}

type Connection struct {
	ClusterID string `json:"clusterID"`
	Version   string `json:"version"`
	LatencyMS int64  `json:"latencyMS"`
}

// Factory is consumed here so connection orchestration can be tested without a cluster.
type Factory interface {
	New(context.Context, *rest.Config) (resource.Client, error)
}

// Sources persists paths only, never the contents of kubeconfig files.
type Sources interface {
	List() ([]string, error)
	Add(string) error
}
