package node

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PreviewCordon changes only scheduling state through the common mutation pipeline.
func (s *Service) PreviewCordon(ctx context.Context, request CordonRequest) (manifest.Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	client, err := s.resolver.NodeClient(request.Target.ClusterID)
	if err != nil {
		return manifest.Preview{}, err
	}
	object, err := readNode(ctx, client, request.Target)
	if err != nil {
		return manifest.Preview{}, err
	}
	if request.Baseline.UID == "" || request.Baseline.ResourceVersion == "" || formedit.Baseline(object) != request.Baseline {
		return manifest.Preview{}, errors.New("节点已变化或被重建，请刷新基线后重新预检")
	}
	if err := unstructured.SetNestedField(object.Object, request.Unschedulable, "spec", "unschedulable"); err != nil {
		return manifest.Preview{}, err
	}
	text, err := manifest.RenderDraft(object)
	if err != nil {
		return manifest.Preview{}, err
	}
	return s.writer.Preview(ctx, manifest.Request{Target: request.Target, Baseline: request.Baseline, Mode: "patch", YAML: text})
}
