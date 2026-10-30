package storage

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

type Service struct {
	resolver Resolver
	writer   Previewer
}

func NewService(resolver Resolver, writer Previewer) *Service {
	return &Service{resolver: resolver, writer: writer}
}

func (s *Service) Claim(ctx context.Context, target resource.Ref) (Claim, error) {
	object, err := s.read(ctx, target, "persistentvolumeclaims", "PersistentVolumeClaim")
	if err != nil {
		return Claim{}, err
	}
	return claimSnapshot(object)
}

func (s *Service) Volume(ctx context.Context, target resource.Ref) (Volume, error) {
	object, err := s.read(ctx, target, "persistentvolumes", "PersistentVolume")
	if err != nil {
		return Volume{}, err
	}
	return volumeSnapshot(object)
}

func (s *Service) Class(ctx context.Context, target resource.Ref) (Class, error) {
	object, err := s.read(ctx, target, "storageclasses", "StorageClass")
	if err != nil {
		return Class{}, err
	}
	return classSnapshot(object)
}

func validateTarget(target resource.Ref, resourceName string) error {
	group := ""
	if resourceName == "storageclasses" {
		group = "storage.k8s.io"
	}
	if target.ClusterID == "" || target.Group != group || target.Version != "v1" || target.Resource != resourceName || len(validation.IsDNS1123Subdomain(target.Name)) != 0 {
		return errors.New("存储资源标识无效")
	}
	if resourceName == "persistentvolumeclaims" {
		if len(validation.IsDNS1123Label(target.Namespace)) != 0 {
			return errors.New("PVC 需要有效命名空间")
		}
	} else if target.Namespace != "" {
		return errors.New("PV 和 StorageClass 必须使用集群作用域")
	}
	return nil
}

func (s *Service) read(ctx context.Context, target resource.Ref, resourceName, kind string) (*unstructured.Unstructured, error) {
	if err := validateTarget(target, resourceName); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	reader, err := s.resolver.StorageReader(target.ClusterID)
	if err != nil {
		return nil, err
	}
	object, err := reader.GetObject(ctx, target)
	if err != nil {
		return nil, err
	}
	apiVersion := target.Version
	if target.Group != "" {
		apiVersion = target.Group + "/" + target.Version
	}
	if object == nil || object.GetKind() != kind || object.GetAPIVersion() != apiVersion || object.GetName() != target.Name || object.GetNamespace() != target.Namespace || object.GetUID() == "" || object.GetResourceVersion() == "" {
		return nil, errors.New("集群未返回完整且匹配的存储资源身份")
	}
	return object, nil
}

func classTarget(target resource.Ref, name string) resource.Ref {
	return resource.Ref{ClusterID: target.ClusterID, Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses", Name: name}
}
