package manifest

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

// Dependency freezes a source object used to prepare a mutation, such as a
// rollback's ReplicaSet. Checks are conservative; any resourceVersion change
// requires a new preview. Kubernetes offers no atomic cross-object transaction.
type Dependency struct {
	Target   resource.Ref
	Baseline Baseline
}

// PreviewGuarded is an internal form boundary; only trusted adapters add guards.
//
//wails:ignore
func (s *Service) PreviewGuarded(ctx context.Context, request Request, dependencies []Dependency) (Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return Preview{}, err
	}
	if err = checkDependencies(ctx, client, request.Target, dependencies); err != nil {
		return Preview{}, err
	}
	operation, before, desired, err := prepare(ctx, client, request)
	if err != nil {
		return Preview{}, err
	}
	return s.previewOperation(ctx, client, operation, before, desired, operation.Mode, dependencies)
}
func checkDependencies(ctx context.Context, client Client, target resource.Ref, dependencies []Dependency) error {
	if len(dependencies) > 4 {
		return errors.New("关联预检对象过多")
	}
	for _, dependency := range dependencies {
		definition := dependency.Target.Group == "apiextensions.k8s.io" && dependency.Target.Version == "v1" && dependency.Target.Resource == "customresourcedefinitions" && dependency.Target.Namespace == ""
		storageClass := dependency.Target.Group == "storage.k8s.io" && dependency.Target.Version == "v1" && dependency.Target.Resource == "storageclasses" && dependency.Target.Namespace == ""
		if dependency.Target.ClusterID != target.ClusterID || (dependency.Target.Namespace != target.Namespace && !definition && !storageClass) {
			return errors.New("关联预检对象必须与操作对象位于同一集群和命名空间")
		}
		object, err := client.GetObject(ctx, dependency.Target)
		if err != nil {
			return err
		}
		if err = checkBaseline(object, dependency.Baseline); err != nil {
			return errors.New("关联对象（发布历史、CRD 定义或 StorageClass）已变化或被重新创建，请刷新后重新预检")
		}
	}
	return nil
}
func (s *Service) previewOperation(ctx context.Context, client Client, operation Operation, before, desired, mode string, dependencies []Dependency) (Preview, error) {
	result, err := client.WriteObject(ctx, operation, true)
	if err != nil {
		return Preview{}, err
	}
	after, err := render(editableObject(result))
	if err != nil {
		return Preview{}, err
	}
	return s.remember(Preview{Target: operation.Target, Mode: mode, Before: before, Desired: desired, After: after}, ticket{operation: operation, connectionID: client.ConnectionID(), dependencies: slices.Clone(dependencies)})
}
