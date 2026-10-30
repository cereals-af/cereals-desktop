package relations

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
)

type Service struct{ resolver Resolver }

func NewService(resolver Resolver) *Service { return &Service{resolver: resolver} }

func (s *Service) Inspect(ctx context.Context, target resource.Ref) (Report, error) {
	if target.ClusterID == "" || target.Version == "" || target.Resource == "" || target.Name == "" {
		return Report{}, errors.New("关联查询需要完整资源标识")
	}
	client, err := s.resolver.RelationClient(target.ClusterID)
	if err != nil {
		return Report{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	root, err := client.GetObject(ctx, target)
	if err != nil {
		return Report{}, err
	}
	if root.GetUID() == "" {
		return Report{}, errors.New("资源未返回 UID，无法可靠关联")
	}
	reader := reader{client: client, target: target}
	sections := []Section{}
	if target.Namespace != "" {
		switch {
		case target.Group == "apps" && target.Resource == "deployments":
			sections = reader.descendants(ctx, root, "apps", "replicasets", "ReplicaSet")
		case target.Group == "batch" && target.Resource == "cronjobs":
			sections = reader.descendants(ctx, root, "batch", "jobs", "Job")
		case target.Group == "apps" && (target.Resource == "replicasets" || target.Resource == "statefulsets" || target.Resource == "daemonsets"),
			target.Group == "batch" && target.Resource == "jobs":
			sections = append(sections, reader.pods(ctx, ownerIDs(root), "由当前资源 UID 管理的 Pod"))
		case target.Group == "" && target.Resource == "services":
			sections = reader.service(ctx, root)
		}
	}
	if target.Group == "" && target.Resource == "persistentvolumeclaims" {
		sections = reader.claimStorage(ctx, root)
	} else if target.Group == "" && target.Resource == "persistentvolumes" {
		sections = reader.volumeStorage(ctx, root)
	} else if target.Group == "storage.k8s.io" && target.Resource == "storageclasses" {
		sections = reader.classStorage(ctx, root)
	}
	if target.Group == "apps" && target.Resource == "statefulsets" {
		sections = append(sections, reader.statefulClaims(ctx, root))
	}
	if target.Group == "" && target.Resource == "nodes" {
		sections = append(sections, reader.nodePods(ctx, root))
	}
	events := reader.events(ctx, root)
	sections = append(sections, events)
	if ctx.Err() != nil {
		return Report{}, ctx.Err()
	}
	return Report{Root: summarize(*root, target, "当前资源"), Sections: sections, CheckedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

type reader struct {
	client Client
	target resource.Ref
}

func (r reader) events(ctx context.Context, root *unstructured.Unstructured) Section {
	section := newSection("events", "当前资源事件")
	section.Note = "按 involvedObject.uid 查询当前资源；历史事件可能已过期。子资源事件可在对应资源的排障视图查看。"
	objects := r.collect(ctx, &section, "", "events", "", fields.OneTermEqualSelector("involvedObject.uid", string(root.GetUID())).String(), nil)
	for _, object := range objects {
		uid, _, _ := unstructured.NestedString(object.Object, "involvedObject", "uid")
		if uid == string(root.GetUID()) {
			section.Events = append(section.Events, eventSummary(object))
		}
	}
	sortEvents(section.Events)
	return section
}

func newSection(key, title string) Section {
	return Section{Key: key, Title: title, Items: []Node{}, Events: []Event{}}
}
