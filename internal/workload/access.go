package workload

import (
	"context"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

// Access is advisory; every mutation is still authorized by the API server.
type Access struct {
	Verb        string `json:"verb"`
	Subresource string `json:"subresource"`
	State       string `json:"state"`
	Message     string `json:"message"`
}
type accessReviewer interface {
	ReviewAccess(context.Context, resource.Ref, string, string) (Access, error)
}

func (s *Service) Permissions(ctx context.Context, target resource.Ref) ([]Access, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return nil, err
	}
	checks := []Access{{Verb: "patch"}, {Verb: "get", Subresource: "scale"}, {Verb: "update", Subresource: "scale"}, {Verb: "patch", Subresource: "scale"}}
	reviewer, ok := client.(accessReviewer)
	for i, check := range checks {
		checks[i].State = "unknown"
		if !ok {
			checks[i].Message = "该连接未提供权限检查"
			continue
		}
		result, err := reviewer.ReviewAccess(ctx, target, check.Verb, check.Subresource)
		if err != nil {
			checks[i].Message = "权限检查不可用：" + err.Error()
			continue
		}
		checks[i] = result
	}
	return checks, nil
}
