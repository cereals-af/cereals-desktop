package resource

import (
	"context"
	"errors"
	"time"
)

type Service struct{ resolver Resolver }

func NewService(resolver Resolver) *Service { return &Service{resolver: resolver} }

func (s *Service) Discover(ctx context.Context, clusterID string) (Catalog, error) {
	client, err := s.resolver.Client(clusterID)
	if err != nil {
		return Catalog{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return client.Discover(ctx)
}

func (s *Service) List(ctx context.Context, query Query) (Page, error) {
	if query.Version == "" || query.Resource == "" {
		return Page{}, errors.New("请选择资源类型")
	}
	client, err := s.resolver.Client(query.ClusterID)
	if err != nil {
		return Page{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return client.List(ctx, query)
}

func (s *Service) YAML(ctx context.Context, ref Ref) (string, error) {
	if ref.Version == "" || ref.Resource == "" || ref.Name == "" {
		return "", errors.New("资源标识不完整")
	}
	client, err := s.resolver.Client(ref.ClusterID)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return client.YAML(ctx, ref)
}
