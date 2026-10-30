package overview

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type Service struct{ resolver Resolver }

func NewService(resolver Resolver) *Service { return &Service{resolver: resolver} }

func (s *Service) Snapshot(ctx context.Context, clusterID string) (Snapshot, error) {
	if clusterID == "" {
		return Snapshot{}, errors.New("请选择已连接的集群")
	}
	client, err := s.resolver.OverviewClient(clusterID)
	if err != nil {
		return Snapshot{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	result := Snapshot{ClusterID: clusterID, Resources: []ResourceCount{}, Warnings: []string{}}
	var nodes sync.WaitGroup
	nodes.Go(func() {
		nodeCtx, stop := context.WithTimeout(ctx, 20*time.Second)
		defer stop()
		value, err := client.OverviewNodes(nodeCtx)
		if err != nil {
			result.NodeError = err.Error()
			return
		}
		result.Nodes = &value
	})
	catalog, err := client.Discover(ctx)
	if err != nil {
		result.Warnings = append(result.Warnings, "资源类型发现失败："+err.Error())
	} else {
		result.Warnings = append(result.Warnings, catalog.Warnings...)
		result.Resources = countResources(ctx, client, catalog.Resources)
	}
	nodes.Wait()
	if ctx.Err() != nil {
		return Snapshot{}, errors.New("集群统计已取消或超时，请重新刷新")
	}
	result.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func countResources(ctx context.Context, client Client, types []resource.Type) []ResourceCount {
	result := make([]ResourceCount, 0, len(types))
	seen := make(map[string]bool)
	for _, kind := range types {
		key := kind.Group + "/" + kind.Resource
		if seen[key] || strings.Contains(kind.Resource, "/") || !slices.Contains(kind.Verbs, "list") {
			continue
		}
		seen[key] = true
		result = append(result, ResourceCount{Type: kind})
	}
	queue := make(chan int, len(result))
	for i := range result {
		queue <- i
	}
	close(queue)
	var workers sync.WaitGroup
	for range min(4, len(result)) {
		workers.Go(func() {
			for i := range queue {
				result[i] = countResource(ctx, client, result[i].Type)
			}
		})
	}
	workers.Wait()
	return result
}

func countResource(ctx context.Context, client Client, kind resource.Type) ResourceCount {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result := ResourceCount{Type: kind}
	count, err := client.CountResources(ctx, kind)
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Count = &count
	}
	return result
}
