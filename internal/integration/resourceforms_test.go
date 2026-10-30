//go:build integration

package integration

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/resourceform"
	"github.com/cereals-af/cereals-desktop/internal/workload"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
)

func formTree(t *testing.T, text string) formvalue.Value {
	t.Helper()
	raw := map[string]interface{}{}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatal(err)
	}
	v, err := formvalue.Encode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestAPIServerResourceForms(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	defer cancel()
	conn, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	res := resolver{conn.(manifest.Client)}
	writer := manifest.NewService(res)
	defer writer.ServiceShutdown()
	forms := resourceform.NewService(res, writer)
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ group, resource, body string }{
		{"batch", "jobs", `"spec":{"parallelism":1,"completions":1,"template":{"spec":{"restartPolicy":"Never","containers":[{"name":"app","image":"busybox"}]}}}`},
		{"batch", "cronjobs", `"spec":{"schedule":"0 * * * *","timeZone":"Asia/Shanghai","jobTemplate":{"spec":{"template":{"spec":{"restartPolicy":"OnFailure","containers":[{"name":"app","image":"busybox"}]}}}}}`},
		{"networking.k8s.io", "ingresses", `"spec":{"ingressClassName":"nginx","rules":[{"host":"app.example.test","http":{"paths":[{"path":"/","pathType":"Prefix","backend":{"service":{"name":"app","port":{"number":80}}}}]}}],"tls":[{"hosts":["app.example.test"],"secretName":"tls-app"}]}`},
		{"networking.k8s.io", "networkpolicies", `"spec":{"podSelector":{"matchLabels":{"app":"api"}},"policyTypes":["Ingress","Egress"],"ingress":[],"egress":[{"to":[{"ipBlock":{"cidr":"10.0.0.0/8","except":["10.1.0.0/16"]}}],"ports":[{"port":53,"protocol":"UDP"}]}]}`},
		{"", "secrets", `"type":"Opaque","immutable":false`},
	}
	for _, tc := range cases {
		t.Run(tc.resource, func(t *testing.T) {
			target := resource.Ref{ClusterID: "test-only", Group: tc.group, Version: "v1", Resource: tc.resource, Namespace: "default", Name: "visual-" + tc.resource}
			api := client.Resource(schema.GroupVersionResource{Group: tc.group, Version: "v1", Resource: tc.resource}).Namespace("default")
			request := resourceform.Request{Target: target, Mode: "create", Value: formTree(t, `{"name":"`+target.Name+`","namespace":"default","annotations":{"external":"keep"},`+tc.body+`}`), Revealed: true, Entries: []resourceform.SecretEntry{{Key: "config", Value: "配置:\n  password: test\n", Encoding: "text"}, {Key: "binary", Value: "AP8=", Encoding: "base64"}}}
			preview, err := forms.Preview(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := api.Get(ctx, target.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
				t.Fatal("dry run persisted")
			}
			commitPreview(t, ctx, writer, preview)
			snap, err := forms.Snapshot(ctx, target, true)
			if err != nil {
				t.Fatal(err)
			}
			request = resourceform.Request{Target: target, Mode: "patch", Value: snap.Value, Baseline: snap.Baseline, Entries: snap.Entries, Revealed: true}
			labels := formvalue.Value{Type: "object", Properties: map[string]formvalue.Value{"edited": {Type: "string", Text: "true"}}}
			request.Value.Properties["labels"] = labels
			preview, err = forms.Preview(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			commitPreview(t, ctx, writer, preview)
			current := getRelease(t, ctx, api, target.Name)
			if current.GetLabels()["edited"] != "true" || current.GetAnnotations()["external"] != "keep" {
				t.Fatal("metadata lost")
			}
			if _, err = forms.Preview(ctx, request); err == nil {
				t.Fatal("stale baseline accepted")
			}
			fresh, err := forms.Snapshot(ctx, target, true)
			if err != nil {
				t.Fatal(err)
			}
			switch tc.resource {
			case "jobs":
				raw, err := formvalue.Decode(fresh.Value)
				if err != nil {
					t.Fatal(err)
				}
				draft := &unstructured.Unstructured{Object: raw.(map[string]interface{})}
				_ = unstructured.SetNestedSlice(draft.Object, []interface{}{map[string]interface{}{"name": "app", "image": "changed"}}, "spec", "template", "spec", "containers")
				fresh.Value, err = formvalue.Encode(draft.Object)
				if err != nil {
					t.Fatal(err)
				}
				_, err = forms.Preview(ctx, resourceform.Request{Target: target, Mode: "patch", Baseline: fresh.Baseline, Value: fresh.Value})
				if err == nil {
					t.Fatal("API accepted immutable Job template")
				}
			case "cronjobs":
				run, err := forms.PreviewRun(ctx, resourceform.RunRequest{Target: target, Baseline: fresh.Baseline, Name: "manual-run"})
				if err != nil {
					t.Fatal(err)
				}
				patchRelease(t, ctx, api, target.Name, `{"spec":{"schedule":"15 * * * *"}}`)
				if _, err = writer.Commit(ctx, run.Token); err == nil {
					t.Fatal("changed CronJob source not guarded")
				}
				newer, err := forms.Snapshot(ctx, target, false)
				if err != nil {
					t.Fatal(err)
				}
				run, err = forms.PreviewRun(ctx, resourceform.RunRequest{Target: target, Baseline: newer.Baseline, Name: "manual-run"})
				if err != nil {
					t.Fatal(err)
				}
				commitPreview(t, ctx, writer, run)
				jobs := client.Resource(schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}).Namespace("default")
				job := getRelease(t, ctx, jobs, "manual-run")
				if len(job.GetOwnerReferences()) != 1 || job.GetOwnerReferences()[0].UID != current.GetUID() {
					t.Fatal("manual job not linked")
				}
			case "networkpolicies":
				ingress, _, _ := unstructured.NestedSlice(current.Object, "spec", "ingress")
				types, _, _ := unstructured.NestedStringSlice(current.Object, "spec", "policyTypes")
				if len(ingress) != 0 || !reflect.DeepEqual(types, []string{"Ingress", "Egress"}) {
					t.Fatal("deny-all policy changed")
				}
			case "secrets":
				hidden, err := forms.Snapshot(ctx, target, false)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range hidden.Entries {
					if entry.Value != "" {
						t.Fatal("hidden secret leaked")
					}
				}
				if len(fresh.Entries) != 2 || fresh.Entries[0].Value != "AP8=" || !strings.Contains(fresh.Entries[1].Value, "配置") {
					t.Fatal("secret encoding lost")
				}
				patchRelease(t, ctx, api, target.Name, `{"immutable":true}`)
				immutable, err := forms.Snapshot(ctx, target, true)
				if err != nil {
					t.Fatal(err)
				}
				immutable.Entries[0].Value = "AQI="
				if _, err = forms.Preview(ctx, resourceform.Request{Target: target, Mode: "patch", Baseline: immutable.Baseline, Value: immutable.Value, Entries: immutable.Entries, Revealed: true}); err == nil {
					t.Fatal("immutable secret changed")
				}
			}
		})
	}
	t.Run("advanced-pod", func(t *testing.T) {
		target := resource.Ref{ClusterID: "test-only", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default", Name: "advanced"}
		options := formTree(t, `{"initContainers":[{"name":"init","image":"busybox","command":["sh","-c","true"]}],"volumes":[{"name":"data","emptyDir":{}}],"tolerations":[{"key":"dedicated","operator":"Exists","effect":"NoSchedule"}],"securityContext":{"runAsNonRoot":true},"imagePullSecrets":[{"name":"registry"}],"affinity":{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"kubernetes.io/os","operator":"In","values":["linux"]}]}]}}}}`)
		advanced := formTree(t, `{"env":[{"name":"TOKEN","valueFrom":{"secretKeyRef":{"name":"credentials","key":"token","optional":true}}}],"envFrom":[{"configMapRef":{"name":"settings","optional":true}}],"volumeMounts":[{"name":"data","mountPath":"/data"}],"securityContext":{"allowPrivilegeEscalation":false}}`)
		service := workload.NewService(res, writer)
		request := workload.Request{Target: target, Action: "create", Form: workload.Deployment{Name: target.Name, Namespace: target.Namespace, Replicas: 1, PodOptions: &options, Containers: []workload.Container{{Name: "app", Image: "busybox", Advanced: &advanced}}}}
		preview, err := service.Preview(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		commitPreview(t, ctx, writer, preview)
		snap, err := service.Deployment(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		delete(snap.Form.Containers[0].Advanced.Properties, "envFrom")
		snap.Form.Containers[0].Image = "busybox:stable"
		preview, err = service.Preview(ctx, workload.Request{Target: target, Action: "update", Baseline: snap.Baseline, Form: snap.Form})
		if err != nil {
			t.Fatal(err)
		}
		commitPreview(t, ctx, writer, preview)
		snap, err = service.Deployment(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		if _, found := snap.Form.Containers[0].Advanced.Properties["envFrom"]; found {
			t.Fatal("envFrom removal failed")
		}
		if len(snap.Form.PodOptions.Properties["initContainers"].Items) != 1 || len(snap.Form.Containers[0].Advanced.Properties["volumeMounts"].Items) != 1 {
			t.Fatal("advanced fields lost")
		}
	})
}
