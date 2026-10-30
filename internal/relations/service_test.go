package relations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type fakeClient struct {
	root *unstructured.Unstructured
	list func(resource.Ref, metav1.ListOptions) (*unstructured.UnstructuredList, error)
}

func (f *fakeClient) GetObject(ctx context.Context, _ resource.Ref) (*unstructured.Unstructured, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.root, nil
}
func (f *fakeClient) ListObjects(ctx context.Context, ref resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.list(ref, options)
}

type fakeResolver struct {
	client    Client
	requested string
}

func (r *fakeResolver) RelationClient(id string) (Client, error) {
	r.requested = id
	return r.client, nil
}

func object(kind, name, uid string, owner *unstructured.Unstructured) unstructured.Unstructured {
	value := unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "kind": kind}}
	value.SetName(name)
	value.SetNamespace("team")
	value.SetUID(types.UID(uid))
	if owner != nil {
		controller := true
		value.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: owner.GetAPIVersion(), Kind: owner.GetKind(), Name: owner.GetName(), UID: owner.GetUID(), Controller: &controller}})
	}
	return value
}
func page(items ...unstructured.Unstructured) *unstructured.UnstructuredList {
	return &unstructured.UnstructuredList{Items: items}
}
func target(group, resourceName, name string) resource.Ref {
	return resource.Ref{ClusterID: "cluster-a", Group: group, Version: "v1", Resource: resourceName, Namespace: "team", Name: name}
}

func TestDeploymentRelationsUseUIDAcrossPagesAndKeepPartialErrors(t *testing.T) {
	root := object("Deployment", "app", "deployment-new", nil)
	oldRoot := object("Deployment", "app", "deployment-old", nil)
	rs := object("ReplicaSet", "app-new", "rs-new", &root)
	foreign := object("ReplicaSet", "app-old", "rs-old", &oldRoot)
	pod := object("Pod", "good", "pod-good", &rs)
	foreignPod := object("Pod", "foreign", "pod-foreign", &foreign)
	otherNS := pod.DeepCopy()
	otherNS.SetNamespace("other")
	resolver := &fakeResolver{client: &fakeClient{root: &root, list: func(ref resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		if ref.Namespace != "team" || options.Limit != 100 {
			t.Fatalf("wrong scope/options: %+v %+v", ref, options)
		}
		switch ref.Resource {
		case "replicasets":
			if options.Continue == "" {
				result := page(foreign)
				result.SetContinue("next")
				return result, nil
			}
			return page(rs), nil
		case "pods":
			return page(pod, foreignPod, *otherNS), nil
		case "events":
			if options.FieldSelector != "involvedObject.uid=deployment-new" {
				t.Fatalf("bad event selector: %s", options.FieldSelector)
			}
			return nil, errors.New("权限不足")
		default:
			t.Fatalf("unexpected resource %s", ref.Resource)
			return nil, nil
		}
	}}}
	report, err := NewService(resolver).Inspect(t.Context(), target("apps", "deployments", "app"))
	if err != nil {
		t.Fatal(err)
	}
	if resolver.requested != "cluster-a" || len(report.Sections) != 3 {
		t.Fatalf("unexpected report %+v", report)
	}
	if len(report.Sections[0].Items) != 1 || report.Sections[0].Items[0].UID != "rs-new" {
		t.Fatal("included foreign owner")
	}
	if len(report.Sections[1].Items) != 1 || report.Sections[1].Items[0].UID != "pod-good" {
		t.Fatal("included foreign Pod")
	}
	if report.Sections[2].Error == "" {
		t.Fatal("permission failure hidden as empty events")
	}
}

func TestSelectorlessServiceDoesNotListAllPodsAndKeepsManualEndpointSlices(t *testing.T) {
	root := object("Service", "service", "service-uid", nil)
	slice := object("EndpointSlice", "manual", "slice-uid", nil)
	slice.SetLabels(map[string]string{"kubernetes.io/service-name": "service"})
	client := &fakeClient{root: &root, list: func(ref resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		switch ref.Resource {
		case "endpointslices":
			if options.LabelSelector != "kubernetes.io/service-name=service" {
				t.Fatalf("selector: %s", options.LabelSelector)
			}
			return page(slice), nil
		case "events":
			return page(), nil
		default:
			t.Fatalf("selectorless service listed %s", ref.Resource)
			return nil, nil
		}
	}}
	report, err := NewService(&fakeResolver{client: client}).Inspect(t.Context(), target("", "services", "service"))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Sections[0].Items) != 1 || !strings.Contains(report.Sections[1].Note, "不执行") {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestPaginationFailureAndScanLimitAreExplicit(t *testing.T) {
	for _, repeat := range []bool{false, true} {
		calls := 0
		client := &fakeClient{list: func(_ resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
			calls++
			if calls > 1 && !repeat {
				return nil, errors.New("later page denied")
			}
			result := page(object("Pod", "item", "uid", nil))
			result.SetContinue("repeated")
			return result, nil
		}}
		section := newSection("pods", "Pod")
		items := (reader{client: client, target: target("", "pods", "item")}).collect(t.Context(), &section, "", "pods", "", "", nil)
		if calls != 2 || len(items) == 0 || (!section.Truncated && section.Error == "") {
			t.Fatalf("incomplete query concealed: %+v", section)
		}
	}
	client := &fakeClient{list: func(_ resource.Ref, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		return &unstructured.UnstructuredList{Items: make([]unstructured.Unstructured, maxScanned+1)}, nil
	}}
	section := newSection("pods", "Pod")
	(reader{client: client}).collect(t.Context(), &section, "", "pods", "", "", nil)
	if !section.Truncated {
		t.Fatal("scan limit ignored")
	}
}

func TestEventUIDFilterAndCancellation(t *testing.T) {
	root := object("Pod", "pod", "new-uid", nil)
	event := object("Event", "event", "event-uid", nil)
	event.Object["involvedObject"] = map[string]interface{}{"uid": "old-uid"}
	client := &fakeClient{root: &root, list: func(_ resource.Ref, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		return page(event), nil
	}}
	service := NewService(&fakeResolver{client: client})
	report, err := service.Inspect(t.Context(), target("", "pods", "pod"))
	if err != nil || len(report.Sections[0].Events) != 0 {
		t.Fatal("event from a replaced resource included")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.Inspect(ctx, target("", "pods", "pod")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
