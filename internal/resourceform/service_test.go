package resourceform

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type client struct {
	manifest.Client
	object *unstructured.Unstructured
}

func (c client) GetObject(ctx context.Context, _ resource.Ref) (*unstructured.Unstructured, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.object, nil
}

type resolver struct{ client manifest.Client }

func (r resolver) ManifestClient(string) (manifest.Client, error) { return r.client, nil }

type capture struct {
	requests     []manifest.Request
	dependencies []manifest.Dependency
}

func (w *capture) Preview(_ context.Context, r manifest.Request) (manifest.Preview, error) {
	w.requests = append(w.requests, r)
	return manifest.Preview{}, nil
}

func (w *capture) PreviewGuarded(ctx context.Context, r manifest.Request, d []manifest.Dependency) (manifest.Preview, error) {
	w.dependencies = d
	return w.Preview(ctx, r)
}

func object(t *testing.T, text string) *unstructured.Unstructured {
	t.Helper()
	o := &unstructured.Unstructured{}
	if err := o.UnmarshalJSON([]byte(text)); err != nil {
		t.Fatal(err)
	}
	return o
}

func value(t *testing.T, raw interface{}) formvalue.Value {
	t.Helper()
	v, err := formvalue.Encode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVisualFormPreservesUnknownFieldsAndRejectsScopeOrStaleBaseline(t *testing.T) {
	o := object(t, `{"apiVersion":"networking.k8s.io/v1","kind":"Ingress","metadata":{"name":"app","namespace":"default","uid":"uid","resourceVersion":"5","finalizers":["keep"],"labels":{"external":"keep"}},"spec":{"ingressClassName":"nginx","future":{"integer":9007199254740993},"defaultBackend":{"service":{"name":"api","port":{"number":80}}}},"status":{"future":"keep"}}`)
	target := resource.Ref{ClusterID: "test", Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Namespace: "default", Name: "app"}
	w := &capture{}
	s := NewService(resolver{client{object: o}}, w)
	snap, err := s.Snapshot(t.Context(), target, false)
	if err != nil {
		t.Fatal(err)
	}
	spec := snap.Value.Properties["spec"]
	spec.Properties["ingressClassName"] = value(t, "internal")
	snap.Value.Properties["spec"] = spec
	req := Request{Target: target, Baseline: snap.Baseline, Mode: "patch", Value: snap.Value}
	if _, err = s.Preview(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	desired := object(t, w.requests[0].YAML)
	n, _, _ := unstructured.NestedInt64(desired.Object, "spec", "future", "integer")
	if n != 9007199254740993 || !reflect.DeepEqual(desired.GetFinalizers(), o.GetFinalizers()) || o.Object["status"] == nil {
		t.Fatal("unexposed fields lost")
	}
	if actual, _, _ := unstructured.NestedString(o.Object, "spec", "ingressClassName"); actual != "nginx" {
		t.Fatal("source mutated")
	}
	req.Baseline.ResourceVersion = "4"
	if _, err = s.Preview(t.Context(), req); err == nil {
		t.Fatal("stale accepted")
	}
	req.Baseline = snap.Baseline
	req.Value.Properties["namespace"] = value(t, "other")
	if _, err = s.Preview(t.Context(), req); err == nil {
		t.Fatal("scope redirect accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = s.Snapshot(ctx, target, false); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
}

func TestSecretRevealRoundTripBinaryAndImmutable(t *testing.T) {
	text := "配置:\n  密码: secret\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	o := object(t, `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"secret","namespace":"default","uid":"s","resourceVersion":"1","annotations":{"kubectl.kubernetes.io/last-applied-configuration":"sensitive"}},"type":"Opaque","data":{"text":"`+encoded+`","binary":"AP8="}}`)
	target := resource.Ref{ClusterID: "test", Version: "v1", Resource: "secrets", Namespace: "default", Name: "secret"}
	w := &capture{}
	s := NewService(resolver{client{object: o}}, w)
	hidden, err := s.Snapshot(t.Context(), target, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := hidden.Value.Properties["annotations"]; exists {
		t.Fatal("annotation leaked")
	}
	for _, entry := range hidden.Entries {
		if entry.Value != "" {
			t.Fatal("secret leaked before reveal")
		}
	}
	shown, err := s.Snapshot(t.Context(), target, true)
	if err != nil {
		t.Fatal(err)
	}
	if shown.Entries[0].Encoding != "base64" || shown.Entries[0].Value != "AP8=" || shown.Entries[1].Value != text {
		t.Fatal("decoding changed bytes")
	}
	req := Request{Target: target, Baseline: shown.Baseline, Mode: "patch", Value: shown.Value, Entries: shown.Entries}
	if _, err = s.Preview(t.Context(), req); err == nil {
		t.Fatal("unrevealed edit accepted")
	}
	req.Revealed = true
	if _, err = s.Preview(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	desired := object(t, w.requests[0].YAML)
	if !reflect.DeepEqual(o.Object["data"], desired.Object["data"]) || !w.requests[0].RevealSecret {
		t.Fatal("secret roundtrip or pipeline changed")
	}
	o.Object["immutable"] = true
	req.Value.Properties["immutable"] = value(t, true)
	req.Entries[1].Value = "new"
	if _, err = s.Preview(t.Context(), req); err == nil {
		t.Fatal("immutable data changed")
	}
	o.Object["immutable"] = false
	req.Entries[1] = req.Entries[0]
	if _, err = s.Preview(t.Context(), req); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatal("duplicate keys accepted")
	}
	req.Entries = []SecretEntry{{Key: "a", Value: "invalid!", Encoding: "base64"}}
	if _, err = s.Preview(t.Context(), req); err == nil || strings.Contains(err.Error(), "invalid!") {
		t.Fatal("bad encoding accepted or leaked")
	}
}

func TestRunFreezesCronJobAndCreatesOwnedJob(t *testing.T) {
	o := object(t, `{"apiVersion":"batch/v1","kind":"CronJob","metadata":{"name":"backup","namespace":"default","uid":"cron","resourceVersion":"3"},"spec":{"jobTemplate":{"metadata":{"labels":{"external":"keep"}},"spec":{"template":{"spec":{"restartPolicy":"Never","containers":[{"name":"backup","image":"busybox"}]}}}}}}`)
	target := resource.Ref{ClusterID: "test", Group: "batch", Version: "v1", Resource: "cronjobs", Namespace: "default", Name: "backup"}
	w := &capture{}
	s := NewService(resolver{client{object: o}}, w)
	if _, err := s.PreviewRun(t.Context(), RunRequest{Target: target, Baseline: formedit.Baseline(o), Name: "manual"}); err != nil {
		t.Fatal(err)
	}
	req := w.requests[0]
	job := object(t, req.YAML)
	if req.Mode != "create" || req.Target.Resource != "jobs" || job.GetKind() != "Job" || job.GetName() != "manual" || job.GetLabels()["external"] != "keep" || len(w.dependencies) != 1 || w.dependencies[0].Baseline != formedit.Baseline(o) {
		t.Fatal("run lost scope or source guard")
	}
	if len(job.GetOwnerReferences()) != 1 || string(job.GetOwnerReferences()[0].UID) != "cron" {
		t.Fatal("owner link missing")
	}
}

func TestEmptyImmutableSecretMetadataAndControlBytes(t *testing.T) {
	object := object(t, `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"empty","namespace":"default","uid":"u","resourceVersion":"1"},"type":"Opaque","immutable":true}`)
	target := resource.Ref{ClusterID: "test", Version: "v1", Resource: "secrets", Name: "empty", Namespace: "default"}
	writer := &capture{}
	service := NewService(resolver{client{object: object}}, writer)
	snapshot, err := service.Snapshot(t.Context(), target, true)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Value.Properties["labels"] = value(t, map[string]interface{}{"edited": "true"})
	if _, err = service.Preview(t.Context(), Request{Target: target, Baseline: snapshot.Baseline, Mode: "patch", Value: snapshot.Value, Entries: snapshot.Entries, Revealed: true}); err != nil {
		t.Fatal(err)
	}
	if readableSecret([]byte{0, 8}) || !readableSecret([]byte("中文\n\t")) {
		t.Fatal("binary classification changed bytes")
	}
}
