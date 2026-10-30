package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type fakeReader struct {
	objects map[string]*unstructured.Unstructured
	denied  string
}

func (r *fakeReader) GetObject(ctx context.Context, ref resource.Ref) (*unstructured.Unstructured, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ref.Resource == r.denied {
		return nil, errors.New("权限不足")
	}
	object := r.objects[ref.Resource]
	if object == nil {
		return nil, errors.New("not found")
	}
	return object.DeepCopy(), nil
}

type fakeResolver struct {
	reader    Reader
	clusterID string
}

func (r *fakeResolver) StorageReader(id string) (Reader, error) {
	r.clusterID = id
	return r.reader, nil
}

type fakeWriter struct {
	request      manifest.Request
	dependencies []manifest.Dependency
	calls        int
}

func (w *fakeWriter) PreviewGuarded(_ context.Context, request manifest.Request, dependencies []manifest.Dependency) (manifest.Preview, error) {
	w.request, w.dependencies = request, dependencies
	w.calls++
	return manifest.Preview{Target: request.Target, Desired: request.YAML, Token: "test-token"}, nil
}
func fixtures(t *testing.T) (*Service, *fakeReader, *fakeWriter, resource.Ref) {
	t.Helper()
	claim, class := &unstructured.Unstructured{}, &unstructured.Unstructured{}
	if err := claim.UnmarshalJSON([]byte(`{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"data","namespace":"team","uid":"claim-uid","resourceVersion":"10","annotations":{"keep":"yes"}},"spec":{"storageClassName":"fast","accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"1Gi"}},"future":{"sequence":9007199254740993}},"status":{"phase":"Bound","capacity":{"storage":"1Gi"}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := class.UnmarshalJSON([]byte(`{"apiVersion":"storage.k8s.io/v1","kind":"StorageClass","metadata":{"name":"fast","uid":"class-uid","resourceVersion":"20"},"provisioner":"test.csi","allowVolumeExpansion":true}`)); err != nil {
		t.Fatal(err)
	}
	reader := &fakeReader{objects: map[string]*unstructured.Unstructured{"persistentvolumeclaims": claim, "storageclasses": class}}
	writer := &fakeWriter{}
	return NewService(&fakeResolver{reader: reader}, writer), reader, writer, resource.Ref{ClusterID: "cluster-a", Version: "v1", Resource: "persistentvolumeclaims", Namespace: "team", Name: "data"}
}
func TestExpansionPreservesFieldsAndFreezesClass(t *testing.T) {
	service, reader, writer, ref := fixtures(t)
	baseline := formedit.Baseline(reader.objects[ref.Resource])
	preview, err := service.PreviewExpand(t.Context(), ExpandRequest{Target: ref, Baseline: baseline, Capacity: "2048Mi"})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"2Gi", "9007199254740993", "keep", "future"} {
		if !strings.Contains(preview.Desired, text) {
			t.Fatalf("lost %s: %s", text, preview.Desired)
		}
	}
	if len(writer.dependencies) != 1 || writer.dependencies[0].Target.ClusterID != ref.ClusterID || writer.dependencies[0].Target.Namespace != "" || writer.dependencies[0].Baseline.UID != "class-uid" {
		t.Fatalf("class guard: %+v", writer.dependencies)
	}
	if writer.request.Baseline != baseline || writer.request.Mode != "patch" {
		t.Fatal("mutation baseline lost")
	}
	current, _, _ := unstructured.NestedString(reader.objects[ref.Resource].Object, "spec", "resources", "requests", "storage")
	if current != "1Gi" {
		t.Fatal("read object mutated")
	}
}
func TestExpansionRejectsUnsafeOrStaleRequests(t *testing.T) {
	for _, name := range []string{"shrink", "equivalent", "zero", "invalid", "pending", "no-class", "class-disabled", "class-denied", "stale", "replaced", "scope", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			s, reader, writer, ref := fixtures(t)
			request := ExpandRequest{Target: ref, Baseline: formedit.Baseline(reader.objects[ref.Resource]), Capacity: "2Gi"}
			ctx := t.Context()
			switch name {
			case "shrink":
				request.Capacity = "500Mi"
			case "equivalent":
				request.Capacity = "1024Mi"
			case "zero":
				request.Capacity = "0"
			case "invalid":
				request.Capacity = "NaN"
			case "pending":
				reader.objects[ref.Resource].Object["status"] = map[string]interface{}{"phase": "Pending"}
			case "no-class":
				unstructured.RemoveNestedField(reader.objects[ref.Resource].Object, "spec", "storageClassName")
			case "class-disabled":
				reader.objects["storageclasses"].Object["allowVolumeExpansion"] = false
			case "class-denied":
				reader.denied = "storageclasses"
			case "stale":
				request.Baseline.ResourceVersion = "9"
			case "replaced":
				request.Baseline.UID = "old-uid"
			case "scope":
				request.Target.Namespace = "elsewhere"
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := s.PreviewExpand(ctx, request); err == nil || writer.calls != 0 {
				t.Fatalf("unsafe write prepared: %v", err)
			}
		})
	}
}
func TestCreationClassSemanticsAndValidation(t *testing.T) {
	for _, mode := range []string{"default", "none", "named"} {
		s, _, writer, ref := fixtures(t)
		form := ClaimForm{Name: ref.Name, Namespace: ref.Namespace, Capacity: "1.5Gi", VolumeMode: "Filesystem", AccessModes: []string{"ReadWriteOnce"}, ClassMode: mode}
		if mode == "named" {
			form.ClassName = "fast"
		}
		preview, err := s.PreviewCreate(t.Context(), CreateRequest{Target: ref, Form: form})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(preview.Desired, "storageClassName") != (mode != "default") || !strings.Contains(preview.Desired, "1536Mi") {
			t.Fatal(preview.Desired)
		}
		if (len(writer.dependencies) == 1) != (mode == "named") {
			t.Fatal("wrong class guard")
		}
	}
	base := ClaimForm{Capacity: "1Gi", VolumeMode: "Block", AccessModes: []string{"ReadWriteOncePod"}, ClassMode: "none"}
	if _, err := claimSpec(base); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*ClaimForm){func(f *ClaimForm) { f.AccessModes = nil }, func(f *ClaimForm) { f.AccessModes = append(f.AccessModes, "ReadWriteOnce") }, func(f *ClaimForm) { f.ClassName = "unexpected" }, func(f *ClaimForm) { f.Capacity = "-1Gi" }, func(f *ClaimForm) { f.VolumeMode = "invalid" }} {
		form := base
		edit(&form)
		if _, err := claimSpec(form); err == nil {
			t.Fatalf("invalid form accepted: %+v", form)
		}
	}
}
func TestPVCReadDoesNotRequireRelatedPermissions(t *testing.T) {
	s, reader, _, ref := fixtures(t)
	reader.denied = "storageclasses"
	snapshot, err := s.Claim(t.Context(), ref)
	if err != nil || snapshot.Requested != "1Gi" || snapshot.Capacity != "1Gi" {
		t.Fatalf("read: %+v %v", snapshot, err)
	}
}
