//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/relations"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/storage"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type storageResolver struct{ reader storage.Reader }

func (r storageResolver) StorageReader(string) (storage.Reader, error) { return r.reader, nil }

func TestAPIServerStorage(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	connection, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := manifest.NewService(resolver{connection.(manifest.Client)})
	defer writer.ServiceShutdown()
	service := storage.NewService(storageResolver{connection.(storage.Reader)}, writer)
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	classes := client.Resource(schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"})
	claims := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}).Namespace("default")
	inspectionCreate(t, ctx, client, "storage.k8s.io", "storageclasses", `{"apiVersion":"storage.k8s.io/v1","kind":"StorageClass","metadata":{"name":"test-csi","annotations":{"storageclass.kubernetes.io/is-default-class":"true"}},"provisioner":"example.test/csi","allowVolumeExpansion":true,"volumeBindingMode":"WaitForFirstConsumer","reclaimPolicy":"Retain"}`, nil)
	ref := resource.Ref{ClusterID: "test-only", Version: "v1", Resource: "persistentvolumeclaims", Namespace: "default", Name: "storage-data"}
	form := storage.ClaimForm{Name: ref.Name, Namespace: ref.Namespace, Capacity: "1Gi", AccessModes: []string{"ReadWriteOnce"}, VolumeMode: "Filesystem", ClassMode: "named", ClassName: "test-csi"}
	preview, err := service.PreviewCreate(ctx, storage.CreateRequest{Target: ref, Form: form})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claims.Get(ctx, ref.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("dry run persisted: %v", err)
	}
	commitPreview(t, ctx, writer, preview)
	t.Run("default-versus-explicit-empty", func(t *testing.T) {
		for _, mode := range []string{"default", "none"} {
			form := form
			form.Name = "storage-" + mode
			form.ClassMode = mode
			form.ClassName = ""
			target := ref
			target.Name = form.Name
			p, err := service.PreviewCreate(ctx, storage.CreateRequest{Target: target, Form: form})
			if err != nil {
				t.Fatal(err)
			}
			commitPreview(t, ctx, writer, p)
			object, err := claims.Get(ctx, target.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			name, found, _ := unstructured.NestedString(object.Object, "spec", "storageClassName")
			if !found || (mode == "default" && name != "test-csi") || (mode == "none" && name != "") {
				t.Fatalf("default semantics: %s %q %v", mode, name, found)
			}
		}
	})
	// Envtest has no provisioner/binder/kubelet. Simulate their object/status writes;
	// actual admission, dry run, JSON Patch preconditions and RBAC remain server-side.
	object, err := claims.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	object.SetAnnotations(map[string]string{"keep": "9007199254740993", "pv.kubernetes.io/bind-completed": "yes"})
	if err := unstructured.SetNestedField(object.Object, "storage-pv", "spec", "volumeName"); err != nil {
		t.Fatal(err)
	}
	object, err = claims.Update(ctx, object, metav1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	object.Object["status"] = map[string]interface{}{"phase": "Bound", "capacity": map[string]interface{}{"storage": "1Gi"}, "accessModes": []interface{}{"ReadWriteOnce"}}
	object, err = claims.UpdateStatus(ctx, object, metav1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pv := inspectionCreate(t, ctx, client, "", "persistentvolumes", `{"apiVersion":"v1","kind":"PersistentVolume","metadata":{"name":"storage-pv"},"spec":{"capacity":{"storage":"1Gi"},"accessModes":["ReadWriteOnce"],"storageClassName":"test-csi","persistentVolumeReclaimPolicy":"Retain","csi":{"driver":"example.test.csi","volumeHandle":"test-only"}}}`, nil)
	pv.Object["spec"].(map[string]interface{})["claimRef"] = map[string]interface{}{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "name": ref.Name, "namespace": ref.Namespace, "uid": string(object.GetUID())}
	if _, err := client.Resource(schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumes"}).Update(ctx, pv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	inspectionCreate(t, ctx, client, "", "pods", `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"storage-consumer","namespace":"default"},"spec":{"containers":[{"name":"app","image":"example.invalid/app","volumeMounts":[{"name":"data","mountPath":"/data"}]}],"volumes":[{"name":"data","persistentVolumeClaim":{"claimName":"storage-data"}}]}}`, nil)
	snapshot, err := service.Claim(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	request := storage.ExpandRequest{Target: ref, Baseline: snapshot.Baseline, Capacity: "2Gi"}
	preview, err = service.PreviewExpand(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.After, "2Gi") {
		t.Fatal(preview.After)
	}
	live, err := service.Claim(ctx, ref)
	if err != nil || live.Requested != "1Gi" {
		t.Fatalf("expansion dry run changed claim: %+v %v", live, err)
	}
	if _, err := classes.Patch(ctx, "test-csi", types.MergePatchType, []byte(`{"allowVolumeExpansion":false}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Commit(ctx, preview.Token); err == nil {
		t.Fatal("class changed after preview but commit accepted")
	}
	if _, err := service.PreviewExpand(ctx, request); err == nil {
		t.Fatal("disabled class allowed")
	}
	if _, err := classes.Patch(ctx, "test-csi", types.MergePatchType, []byte(`{"allowVolumeExpansion":true}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	preview, err = service.PreviewExpand(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claims.Patch(ctx, ref.Name, types.MergePatchType, []byte(`{"metadata":{"annotations":{"concurrent":"keep"}}}`), metav1.PatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Commit(ctx, preview.Token); err == nil {
		t.Fatal("concurrent claim edit overwritten")
	}
	live, err = service.Claim(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	request.Baseline = live.Baseline
	preview, err = service.PreviewExpand(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	commitPreview(t, ctx, writer, preview)
	live, err = service.Claim(ctx, ref)
	if err != nil || live.Requested != "2Gi" || live.Capacity != "1Gi" {
		t.Fatalf("request/completion conflated: %+v %v", live, err)
	}
	object, err = claims.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if object.GetAnnotations()["keep"] != "9007199254740993" || object.GetAnnotations()["concurrent"] != "keep" {
		t.Fatal("metadata lost")
	}
	request.Baseline = live.Baseline
	request.Capacity = "1Gi"
	if _, err := service.PreviewExpand(ctx, request); err == nil {
		t.Fatal("shrink accepted")
	}
	if _, err := claims.Patch(ctx, ref.Name, types.MergePatchType, []byte(`{"spec":{"resources":{"requests":{"storage":"1Gi"}}}}`), metav1.PatchOptions{DryRun: []string{metav1.DryRunAll}}); err == nil {
		t.Fatal("server accepted shrink")
	}
	checkStorageRelations(t, ctx, connection, service, ref)
	checkStoragePermissions(t, ctx, config, client, ref)
}

func checkStorageRelations(t *testing.T, ctx context.Context, connection resource.Client, service *storage.Service, ref resource.Ref) {
	t.Helper()
	report, err := relations.NewService(inspectionResolver{relations: connection.(relations.Client)}).Inspect(ctx, ref)
	if err != nil || len(report.Sections[0].Items) != 1 || len(report.Sections[1].Items) != 1 || len(report.Sections[2].Items) != 1 {
		t.Fatalf("storage relations: %+v %v", report, err)
	}
	pvRef := resource.Ref{ClusterID: ref.ClusterID, Version: "v1", Resource: "persistentvolumes", Name: "storage-pv"}
	pv, err := service.Volume(ctx, pvRef)
	if err != nil || pv.Driver != "example.test.csi" || pv.ReclaimPolicy != "Retain" {
		t.Fatalf("PV: %+v %v", pv, err)
	}
	scRef := resource.Ref{ClusterID: ref.ClusterID, Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses", Name: "test-csi"}
	sc, err := service.Class(ctx, scRef)
	if err != nil || !sc.Default || !sc.AllowExpansion || sc.BindingMode != "WaitForFirstConsumer" {
		t.Fatalf("SC: %+v %v", sc, err)
	}
	report, err = relations.NewService(inspectionResolver{relations: connection.(relations.Client)}).Inspect(ctx, scRef)
	if err != nil || len(report.Sections[0].Items) != 2 || len(report.Sections[1].Items) != 1 {
		t.Fatalf("SC relations: %+v %v", report, err)
	}
}

func checkStoragePermissions(t *testing.T, ctx context.Context, config *rest.Config, client dynamic.Interface, ref resource.Ref) {
	t.Helper()
	inspectionCreate(t, ctx, client, "rbac.authorization.k8s.io", "roles", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"Role","metadata":{"name":"storage-reader","namespace":"default"},"rules":[{"apiGroups":[""],"resources":["persistentvolumeclaims"],"verbs":["get"]}]}`, nil)
	inspectionCreate(t, ctx, client, "rbac.authorization.k8s.io", "rolebindings", `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"RoleBinding","metadata":{"name":"storage-reader","namespace":"default"},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"storage-reader"},"subjects":[{"kind":"User","name":"storage-user","apiGroup":"rbac.authorization.k8s.io"}]}`, nil)
	limited := rest.CopyConfig(config)
	limited.Impersonate = rest.ImpersonationConfig{UserName: "storage-user"}
	connection, err := (kube.Factory{}).New(ctx, limited)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	writer := manifest.NewService(resolver{connection.(manifest.Client)})
	defer writer.ServiceShutdown()
	service := storage.NewService(storageResolver{connection.(storage.Reader)}, writer)
	claim, err := service.Claim(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewExpand(ctx, storage.ExpandRequest{Target: ref, Baseline: claim.Baseline, Capacity: "3Gi"}); err == nil || !strings.Contains(err.Error(), "权限不足") {
		t.Fatalf("expansion permission: %v", err)
	}
	report, err := relations.NewService(inspectionResolver{relations: connection.(relations.Client)}).Inspect(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range report.Sections {
		if section.Error == "" {
			t.Fatalf("denied relation concealed: %+v", section)
		}
	}
	form := storage.ClaimForm{Name: "denied-create", Namespace: "default", Capacity: "1Gi", AccessModes: []string{"ReadWriteOnce"}, VolumeMode: "Filesystem", ClassMode: "none"}
	target := ref
	target.Name = form.Name
	if _, err := service.PreviewCreate(ctx, storage.CreateRequest{Target: target, Form: form}); err == nil || !strings.Contains(err.Error(), "权限不足") {
		t.Fatalf("create permission: %v", err)
	}
}
