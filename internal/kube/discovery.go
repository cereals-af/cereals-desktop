package kube

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (c *Client) Discover(ctx context.Context) (resource.Catalog, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	catalog := resource.Catalog{Resources: []resource.Type{}, Warnings: []string{}}
	versions := []string{"v1"}
	var groups metav1.APIGroupList
	if err := c.rest.Get().AbsPath("/apis").Do(ctx).Into(&groups); err != nil {
		catalog.Warnings = append(catalog.Warnings, "扩展 API 发现失败："+requestError(err).Error())
	}
	for _, group := range groups.Groups {
		if group.PreferredVersion.GroupVersion != "" {
			versions = append(versions, group.PreferredVersion.GroupVersion)
		}
	}
	c.discoverVersions(ctx, versions, &catalog)
	if ctx.Err() != nil {
		return resource.Catalog{}, requestError(ctx.Err())
	}
	if len(catalog.Resources) == 0 {
		return resource.Catalog{}, fmt.Errorf("无法发现可浏览资源：%s", strings.Join(catalog.Warnings, "；"))
	}
	sort.Slice(catalog.Resources, func(i, j int) bool {
		a, b := catalog.Resources[i], catalog.Resources[j]
		return a.Group+"/"+a.Resource < b.Group+"/"+b.Resource
	})
	sort.Strings(catalog.Warnings)
	return catalog, nil
}

func (c *Client) discoverVersions(ctx context.Context, versions []string, catalog *resource.Catalog) {
	var mu sync.Mutex
	var workers sync.WaitGroup
	queue := make(chan string, len(versions))
	for _, version := range versions {
		queue <- version
	}
	close(queue)
	for range min(4, len(versions)) {
		workers.Go(func() {
			for version := range queue {
				if ctx.Err() != nil {
					return
				}
				items, err := c.discoverVersion(ctx, version)
				mu.Lock()
				if err != nil {
					catalog.Warnings = append(catalog.Warnings, version+"："+requestError(err).Error())
				} else {
					catalog.Resources = append(catalog.Resources, items...)
				}
				mu.Unlock()
			}
		})
	}
	workers.Wait()
}

func (c *Client) discoverVersion(ctx context.Context, version string) ([]resource.Type, error) {
	path := "/apis/" + version
	if version == "v1" {
		path = "/api/v1"
	}
	var list metav1.APIResourceList
	if err := c.rest.Get().AbsPath(path).Do(ctx).Into(&list); err != nil {
		return nil, err
	}
	gv, err := schema.ParseGroupVersion(version)
	if err != nil {
		return nil, err
	}
	items := make([]resource.Type, 0, len(list.APIResources))
	for _, item := range list.APIResources {
		if strings.Contains(item.Name, "/") || !slices.Contains(item.Verbs, "list") {
			continue
		}
		items = append(items, resource.Type{Group: gv.Group, Version: gv.Version, Resource: item.Name,
			Kind: item.Kind, Namespaced: item.Namespaced, Verbs: []string(item.Verbs)})
	}
	return items, nil
}
