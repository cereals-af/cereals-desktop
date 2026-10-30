package kube

import (
	"context"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/observe"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

var _ observe.Client = (*Client)(nil)

func (c *Client) Observe(ctx context.Context, query resource.Query, version string, notify func(observe.Change)) error {
	ctx, cancel := context.WithCancel(ctx)
	var children sync.WaitGroup
	defer func() { cancel(); children.Wait() }()
	for _, child := range workloadWatchQueries(query) {
		children.Go(func() {
			// Child watches are best effort: restricted child access must not
			// disable the parent's watch. List results retain verification notes.
			_ = c.observeResource(ctx, child, "", func(change observe.Change) {
				if change.Dirty {
					notify(observe.Change{Dirty: true})
				}
			})
		})
	}
	return c.observeResource(ctx, query, version, notify)
}

func workloadWatchQueries(query resource.Query) []resource.Query {
	if !((query.Group == "apps" && (query.Resource == "deployments" || query.Resource == "statefulsets" || query.Resource == "daemonsets" || query.Resource == "replicasets")) || (query.Group == "batch" && (query.Resource == "jobs" || query.Resource == "cronjobs"))) {
		return nil
	}
	children := []resource.Query{{ClusterID: query.ClusterID, Version: "v1", Resource: "pods", Namespace: query.Namespace}}
	if query.Resource == "cronjobs" {
		children = append(children, resource.Query{ClusterID: query.ClusterID, Group: "batch", Version: "v1", Resource: "jobs", Namespace: query.Namespace})
	}
	return children
}

func (c *Client) observeResource(ctx context.Context, query resource.Query, version string, notify func(observe.Change)) error {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	gvr := schema.GroupVersionResource{Group: query.Group, Version: query.Version, Resource: query.Resource}
	api := c.dynamic.Resource(gvr).Namespace(query.Namespace)
	delay := time.Second
	for ctx.Err() == nil {
		started := time.Now()
		next, err := watchOnce(ctx, api, query.LabelSelector, version, notify)
		version = next
		if ctx.Err() != nil {
			break
		}
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) {
			return requestError(err)
		}
		if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) {
			version = ""
			notify(observe.Change{State: "reconnecting", Dirty: true, Message: "历史版本已过期，正在重新同步列表"})
		} else {
			notify(observe.Change{State: "reconnecting", Message: "实时连接已中断，正在重连"})
		}
		if time.Since(started) > 10*time.Second {
			delay = time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
		delay = min(delay*2, 15*time.Second)
	}
	return requestError(ctx.Err())
}

func watchOnce(ctx context.Context, api dynamic.ResourceInterface, label, version string, notify func(observe.Change)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if version == "" {
		listCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		list, err := api.List(listCtx, metav1.ListOptions{Limit: 1, LabelSelector: label})
		cancel()
		if err != nil {
			return "", err
		}
		version = list.GetResourceVersion()
		// There may have been changes while history was unavailable.
		notify(observe.Change{State: "reconnecting", Dirty: true, Message: "列表已重新同步"})
	}
	seconds := int64(30)
	stream, err := api.Watch(ctx, metav1.ListOptions{ResourceVersion: version, LabelSelector: label, AllowWatchBookmarks: true, TimeoutSeconds: &seconds})
	if err != nil {
		return version, err
	}
	defer stream.Stop()
	notify(observe.Change{State: "live"})
	for {
		select {
		case <-ctx.Done():
			return version, ctx.Err()
		case event, ok := <-stream.ResultChan():
			if !ok {
				return version, nil
			}
			if event.Type == watch.Error {
				return version, apierrors.FromObject(event.Object)
			}
			metadata, err := meta.Accessor(event.Object)
			if err != nil {
				continue
			}
			if metadata.GetResourceVersion() != "" {
				version = metadata.GetResourceVersion()
			}
			if event.Type == watch.Added || event.Type == watch.Modified || event.Type == watch.Deleted {
				notify(observe.Change{State: "live", Dirty: true})
			}
		}
	}
}
