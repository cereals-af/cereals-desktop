package metrics

import (
	"context"
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
)

type Service struct{ resolver Resolver }

func NewService(resolver Resolver) *Service { return &Service{resolver: resolver} }

func (s *Service) Snapshot(ctx context.Context, query Query) (Snapshot, error) {
	if err := validate(query); err != nil {
		return Snapshot{}, err
	}
	reader, err := s.resolver.MetricsReader(query.ClusterID)
	if err != nil {
		return Snapshot{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	return reader.ResourceUsage(ctx, query)
}

func validate(query Query) error {
	if query.ClusterID == "" || (query.Resource != "pods" && query.Resource != "nodes") {
		return errors.New("请选择已连接集群中的 Pod 或 Node")
	}
	if len(query.Targets) == 0 || len(query.Targets) > 1000 {
		return errors.New("单次用量查询需包含 1 至 1000 个资源")
	}
	if query.Namespace != "" && len(validation.IsDNS1123Label(query.Namespace)) != 0 {
		return errors.New("命名空间无效")
	}
	for _, target := range query.Targets {
		if target.UID == "" || len(validation.IsDNS1123Subdomain(target.Name)) != 0 {
			return errors.New("资源名称或 UID 无效")
		}
		if query.Resource == "nodes" {
			if query.Namespace != "" || target.Namespace != "" {
				return errors.New("Node 为集群作用域资源")
			}
		} else if target.Namespace == "" || len(validation.IsDNS1123Label(target.Namespace)) != 0 || (query.Namespace != "" && query.Namespace != target.Namespace) {
			return errors.New("Pod 命名空间与查询范围不匹配")
		}
	}
	return nil
}
