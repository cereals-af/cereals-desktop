//go:build integration

package integration

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/relations"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workload"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func TestAPIServerControllers(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	connection, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := manifest.NewService(resolver{connection.(manifest.Client)})
	defer writer.ServiceShutdown()
	service := workload.NewService(resolver{connection.(manifest.Client)}, writer)
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"statefulsets", "daemonsets", "replicasets"} {
		t.Run(kind, func(t *testing.T) {
			target := resource.Ref{ClusterID: "test-only", Group: "apps", Version: "v1", Resource: kind, Namespace: "default", Name: "controller-" + kind}
			api := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: kind}).Namespace("default")
			form := workload.ControllerForm{Workload: workload.Deployment{Name: target.Name, Namespace: target.Namespace, Replicas: 2, Containers: []workload.Container{{Name: "app", Image: "nginx:1.29"}}}, Strategy: "RollingUpdate", MaxUnavailable: "1", MaxSurge: "0", ServiceName: "headless", PodManagementPolicy: "OrderedReady", RetainDeleted: "Retain", RetainScaled: "Delete", NodeSelector: []workload.Label{{Key: "kubernetes.io/os", Value: "linux"}}, Claims: []workload.ClaimTemplate{}}
			if kind == "statefulsets" {
				form.Claims = []workload.ClaimTemplate{{Name: "data", Capacity: "1Gi", ClassMode: "none", AccessModes: []string{"ReadWriteOnce"}, VolumeMode: "Filesystem", MountPath: "/data"}}
			}
			request := workload.ControllerRequest{Target: target, Action: "create", Form: form}
			preview, err := service.PreviewController(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := api.Get(ctx, target.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
				t.Fatalf("create dry run persisted: %v", err)
			}
			commitPreview(t, ctx, writer, preview)
			// API defaulting is real; no controller/kubelet is running in this control plane.
			patchRelease(t, ctx, api, target.Name, `{"metadata":{"annotations":{"external":"keep"}},"spec":{"template":{"spec":{"terminationGracePeriodSeconds":9007199254740993,"tolerations":[{"key":"keep","operator":"Exists"}]}}}}`)
			before := getRelease(t, ctx, api, target.Name)
			snapshot, err := service.Controller(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Form.Workload.Containers[0].Image = "nginx:1.30"
			request = workload.ControllerRequest{Target: target, Action: "update", Baseline: snapshot.Baseline, Form: snapshot.Form}
			preview, err = service.PreviewController(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, getRelease(t, ctx, api, target.Name)) {
				t.Fatal("update dry run persisted")
			}
			commitPreview(t, ctx, writer, preview)
			after := getRelease(t, ctx, api, target.Name)
			grace, _, _ := unstructured.NestedInt64(after.Object, "spec", "template", "spec", "terminationGracePeriodSeconds")
			tolerations, _, _ := unstructured.NestedSlice(after.Object, "spec", "template", "spec", "tolerations")
			if grace != 9007199254740993 || len(tolerations) != 1 || after.GetAnnotations()["external"] != "keep" {
				t.Fatal("form lost unexposed fields")
			}
			if _, err = service.PreviewController(ctx, request); err == nil {
				t.Fatal("stale form baseline accepted")
			}
			if kind == "daemonsets" {
				if _, found, _ := unstructured.NestedFieldNoCopy(after.Object, "spec", "replicas"); found {
					t.Fatal("daemonset acquired replicas")
				}
			} else {
				scale, err := writer.Scale(ctx, target)
				if err != nil {
					t.Fatal(err)
				}
				p, err := writer.PreviewScale(ctx, manifest.ScaleRequest{Target: target, Baseline: scale.Baseline, Replicas: 0, Method: "update"})
				if err != nil {
					t.Fatal(err)
				}
				commitPreview(t, ctx, writer, p)
			}
			if kind == "statefulsets" {
				current, err := service.Controller(ctx, target)
				if err != nil {
					t.Fatal(err)
				}
				current.Form.Claims[0].Capacity = "2Gi"
				if _, err := service.PreviewController(ctx, workload.ControllerRequest{Target: target, Action: "update", Baseline: current.Baseline, Form: current.Form}); err == nil {
					t.Fatal("immutable claim template edited")
				}
				checkControllerRelations(t, ctx, client, connection.(relations.Client), getRelease(t, ctx, api, target.Name), target)
			}
			if kind != "replicasets" {
				checkControllerRollback(t, ctx, client, api, service, writer, target)
			} else {
				if _, err := service.ControllerHistory(ctx, target); err == nil {
					t.Fatal("RS fabricated history")
				}
			}
		})
	}
}
func checkControllerRollback(t *testing.T, ctx context.Context, client dynamic.Interface, api dynamic.ResourceInterface, service *workload.Service, writer *manifest.Service, target resource.Ref) {
	t.Helper()
	object := getRelease(t, ctx, api, target.Name)
	template, _, _ := unstructured.NestedMap(object.Object, "spec", "template")
	template["$patch"] = "replace"
	_ = unstructured.SetNestedField(template, "historical", "metadata", "annotations", "restored")
	history := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "apps/v1", "kind": "ControllerRevision", "metadata": map[string]interface{}{"name": target.Name + "-revision", "namespace": target.Namespace}, "revision": int64(9007199254740993), "data": map[string]interface{}{"spec": map[string]interface{}{"template": template}}}}
	controller := true
	history.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: object.GetKind(), Name: object.GetName(), UID: object.GetUID(), Controller: &controller}})
	historyAPI := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "controllerrevisions"}).Namespace(target.Namespace)
	history, err := historyAPI.Create(ctx, history, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	report, err := service.ControllerHistory(ctx, target)
	if err != nil || len(report.Revisions) != 1 || report.Revisions[0].Revision != "9007199254740993" {
		t.Fatalf("history: %+v %v", report, err)
	}
	request := workload.ControllerRollbackRequest{Target: target, Baseline: formedit.Baseline(object), Name: history.GetName(), Source: formedit.Baseline(history), Revision: "9007199254740993"}
	preview, err := service.PreviewControllerRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	patchRelease(t, ctx, historyAPI, history.GetName(), `{"metadata":{"annotations":{"concurrent":"changed"}}}`)
	if _, err := writer.Commit(ctx, preview.Token); err == nil {
		t.Fatal("changed history committed")
	}
	request.Source = formedit.Baseline(getRelease(t, ctx, historyAPI, history.GetName()))
	preview, err = service.PreviewControllerRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	patchRelease(t, ctx, api, target.Name, `{"metadata":{"annotations":{"concurrent":"changed"}}}`)
	if _, err = writer.Commit(ctx, preview.Token); err == nil {
		t.Fatal("changed target committed")
	}
	object = getRelease(t, ctx, api, target.Name)
	request.Baseline = formedit.Baseline(object)
	preview, err = service.PreviewControllerRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.After, "9007199254740993") {
		t.Fatal("int64 precision lost")
	}
	commitPreview(t, ctx, writer, preview)
	beforeSpec, _, _ := unstructured.NestedMap(object.Object, "spec")
	after := getRelease(t, ctx, api, target.Name)
	afterSpec, _, _ := unstructured.NestedMap(after.Object, "spec")
	delete(beforeSpec, "template")
	delete(afterSpec, "template")
	if !reflect.DeepEqual(beforeSpec, afterSpec) {
		t.Fatal("rollback changed strategy, replicas or claims")
	}
	// OnDelete restart changes only the template annotation and does not claim Pod replacement.
	snapshot, err := service.Controller(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Form.Strategy = "OnDelete"
	p, err := service.PreviewController(ctx, workload.ControllerRequest{Target: target, Baseline: snapshot.Baseline, Action: "update", Form: snapshot.Form})
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, writer, p)
	snapshot, err = service.Controller(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	p, err = service.PreviewController(ctx, workload.ControllerRequest{Target: target, Baseline: snapshot.Baseline, Action: "restart", Form: snapshot.Form})
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, writer, p)
	after = getRelease(t, ctx, api, target.Name)
	strategy, _, _ := unstructured.NestedString(after.Object, "spec", "updateStrategy", "type")
	if strategy != "OnDelete" {
		t.Fatal("restart changed update strategy")
	}
	request.Baseline = formedit.Baseline(after)
	preview, err = service.PreviewControllerRollback(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err = historyAPI.Delete(ctx, history.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Commit(ctx, preview.Token); err == nil {
		t.Fatal("deleted history committed")
	}
}
func checkControllerRelations(t *testing.T, ctx context.Context, client dynamic.Interface, reader relations.Client, owner *unstructured.Unstructured, target resource.Ref) {
	t.Helper()
	inspectionCreate(t, ctx, client, "", "pods", `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"owned-controller-pod","namespace":"default"},"spec":{"containers":[{"name":"app","image":"nginx"}]}}`, owner)
	inspectionCreate(t, ctx, client, "", "pods", `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"lookalike-controller-pod","namespace":"default","labels":{"app.kubernetes.io/name":"`+target.Name+`"}},"spec":{"containers":[{"name":"app","image":"nginx"}]}}`, nil)
	inspectionCreate(t, ctx, client, "", "persistentvolumeclaims", `{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"data-`+target.Name+`-0","namespace":"default"},"spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"1Gi"}}}}`, nil)
	report, err := relations.NewService(inspectionResolver{relations: reader}).Inspect(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	foundPods, foundClaims := false, false
	for _, section := range report.Sections {
		switch section.Key {
		case "pods":
			foundPods = true
			if len(section.Items) != 1 || section.Items[0].Target.Name != "owned-controller-pod" || section.Items[0].Pod == nil {
				t.Fatalf("owner match: %+v", section)
			}
		case "claims":
			foundClaims = true
			if len(section.Items) != 1 {
				t.Fatalf("claims: %+v", section)
			}
		}
	}
	if !foundPods || !foundClaims {
		t.Fatal("missing workload relations")
	}
}
