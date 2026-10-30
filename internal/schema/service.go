// Package schema supplies cluster-specific schemas for advisory editor support.
package schema

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/util/validation"
)

type Document struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	JSON       string   `json:"json"`
	Source     string   `json:"source"`
	Warnings   []string `json:"warnings"`
}

type Client interface {
	ResourceSchema(context.Context, resource.Ref) (Document, error)
}

type Resolver interface{ SchemaClient(string) (Client, error) }

type Service struct{ resolver Resolver }

func NewService(resolver Resolver) *Service { return &Service{resolver: resolver} }

func (s *Service) Get(ctx context.Context, target resource.Ref) (Document, error) {
	if target.ClusterID == "" || len(validation.IsDNS1123Label(target.Version)) != 0 ||
		len(validation.IsDNS1123Label(target.Resource)) != 0 ||
		(target.Group != "" && len(validation.IsDNS1123Subdomain(target.Group)) != 0) {
		return Document{}, errors.New("Schema 资源类型无效")
	}
	client, err := s.resolver.SchemaClient(target.ClusterID)
	if err != nil {
		return Document{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return client.ResourceSchema(ctx, target)
}
