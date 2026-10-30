// Package kube implements the resource contract with official Kubernetes clients.
package kube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

type Factory struct{}

type Client struct {
	connectionID string
	lifetime     context.Context
	http         *http.Client
	dynamic      dynamic.Interface
	metadata     metadata.Interface
	rest         rest.Interface
	config       *rest.Config
	core         coreclient.CoreV1Interface
}

func (Factory) New(ctx context.Context, config *rest.Config) (resource.Client, error) {
	copy := rest.CopyConfig(config)
	// Individual services set request deadlines. A watch stream must not inherit
	// the short timeout used by the initial connection probe.
	copy.Timeout = 0
	httpClient, err := rest.HTTPClientFor(copy)
	if err != nil {
		return nil, requestError(err)
	}
	dynamicClient, err := dynamic.NewForConfigAndClient(copy, httpClient)
	if err != nil {
		return nil, requestError(err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(copy, httpClient)
	if err != nil {
		return nil, requestError(err)
	}
	metadataClient, err := metadata.NewForConfigAndClient(copy, httpClient)
	if err != nil {
		return nil, requestError(err)
	}
	core, err := coreclient.NewForConfigAndClient(copy, httpClient)
	if err != nil {
		return nil, requestError(err)
	}
	return &Client{config: copy, core: core, connectionID: uuid.NewString(), lifetime: ctx, http: httpClient, dynamic: dynamicClient, metadata: metadataClient, rest: discoveryClient.RESTClient()}, nil
}

func (c *Client) ConnectionID() string { return c.connectionID }

func (c *Client) requestContext(ctx context.Context) (context.Context, func()) {
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(c.lifetime, cancel)
	if c.lifetime.Err() != nil {
		cancel()
	}
	return child, func() { stop(); cancel() }
}

func (c *Client) Version(ctx context.Context) (string, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var info version.Info
	data, err := c.rest.Get().AbsPath("/version").Do(ctx).Raw()
	if err != nil {
		return "", requestError(err)
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", requestError(err)
	}
	if info.GitVersion == "" {
		return "", errors.New("API Server 未返回有效的 Kubernetes 版本")
	}
	return info.GitVersion, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }
