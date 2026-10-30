// Package network adapts Kubernetes network resources to visual forms.
package network

import (
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type Selector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Port struct {
	// OriginalKey identifies the original name (or unnamed port/protocol); empty adds a port.
	OriginalKey string `json:"originalKey"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Port        int32  `json:"port"`
	TargetPort  string `json:"targetPort"`
	NodePort    int32  `json:"nodePort"`
	AppProtocol string `json:"appProtocol"`
}

type ServiceForm struct {
	Name                     string     `json:"name"`
	Namespace                string     `json:"namespace"`
	Type                     string     `json:"type"`
	Headless                 bool       `json:"headless"`
	ExternalName             string     `json:"externalName"`
	Selector                 []Selector `json:"selector"`
	Ports                    []Port     `json:"ports"`
	ExternalTrafficPolicy    string     `json:"externalTrafficPolicy"`
	SessionAffinity          string     `json:"sessionAffinity"`
	PublishNotReadyAddresses bool       `json:"publishNotReadyAddresses"`
}

type Snapshot struct {
	Form                  ServiceForm       `json:"form"`
	Baseline              manifest.Baseline `json:"baseline"`
	ClusterIPs            []string          `json:"clusterIPs"`
	IPFamilies            []string          `json:"ipFamilies"`
	LoadBalancerAddresses []string          `json:"loadBalancerAddresses"`
}

type Request struct {
	Target   resource.Ref      `json:"target"`
	Baseline manifest.Baseline `json:"baseline"`
	Mode     string            `json:"mode"`
	Form     ServiceForm       `json:"form"`
}
