package kube

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/metrics"
	corev1 "k8s.io/api/core/v1"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetricsBatchUsesAdvertisedVersionAndChecksIdentity(t *testing.T) {
	for _, version := range []string{"v1", "v1beta1"} {
		t.Run(version, func(t *testing.T) {
			now := time.Now().UTC()
			requests := 0
			client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				switch r.URL.Path {
				case "/apis/metrics.k8s.io":
					writeJSON(t, w, fmt.Sprintf(`{"preferredVersion":{"version":%q}}`, version))
				case "/apis/metrics.k8s.io/" + version + "/namespaces/team/pods":
					writeJSON(t, w, fmt.Sprintf(`{"items":[
					{"metadata":{"name":"app","namespace":"team"},"timestamp":%q,"window":"15s","containers":[{"name":"a","usage":{"cpu":"250000000n","memory":"256Mi"}},{"name":"sidecar","usage":{"cpu":"50m","memory":"64Mi"}}]},
					{"metadata":{"name":"replaced","namespace":"team"},"timestamp":%q,"window":"15s","containers":[{"name":"a","usage":{"cpu":"1","memory":"1Gi"}}]}]}`, now.Format(time.RFC3339), now.Format(time.RFC3339)))
				case "/api/v1/namespaces/team/pods":
					if !strings.Contains(r.Header.Get("Accept"), "PartialObjectMetadataList") {
						t.Error("identity check fetched full objects")
					}
					if r.URL.Query().Get("continue") == "" {
						writeJSON(t, w, `{"apiVersion":"meta.k8s.io/v1","kind":"PartialObjectMetadataList","metadata":{"continue":"next"},"items":[{"metadata":{"name":"app","namespace":"team","uid":"app-uid","creationTimestamp":"2026-01-01T00:00:00Z"}}]}`)
					} else {
						writeJSON(t, w, `{"apiVersion":"meta.k8s.io/v1","kind":"PartialObjectMetadataList","items":[{"metadata":{"name":"replaced","namespace":"team","uid":"new-uid"}},{"metadata":{"name":"empty","namespace":"team","uid":"empty-uid"}}]}`)
					}
				default:
					t.Errorf("unexpected endpoint %s", r.URL)
				}
			})
			query := metrics.Query{Resource: "pods", Namespace: "team", Targets: []metrics.Target{{Name: "app", Namespace: "team", UID: "app-uid"}, {Name: "replaced", Namespace: "team", UID: "old-uid"}, {Name: "empty", Namespace: "team", UID: "empty-uid"}}}
			result, err := client.(metrics.Reader).ResourceUsage(t.Context(), query)
			if err != nil || result.State != "available" || len(result.Items) != 3 || requests != 4 {
				t.Fatalf("result=%+v requests=%d err=%v", result, requests, err)
			}
			if got := result.Items[0].Usage; got == nil || got.CPUCores != 0.3 || got.MemoryBytes != 320*1024*1024 {
				t.Fatalf("incorrect container total: %+v", got)
			}
			if result.Items[1].State != "replaced" || result.Items[1].Usage != nil || result.Items[2].State != "missing" || result.Items[2].Usage != nil {
				t.Fatal("unknown/replacement sample was displayed as current usage")
			}
		})
	}
}

func TestMetricSampleRejectsStaleIncompleteAndOldIncarnation(t *testing.T) {
	now := time.Now()
	identity := metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{UID: "uid", CreationTimestamp: metav1.NewTime(now.Add(-time.Hour))}}
	target := metrics.Target{Name: "node", UID: "uid"}
	for _, mode := range []string{"current", "zero", "stale", "future", "before-creation", "incomplete", "negative"} {
		t.Run(mode, func(t *testing.T) {
			row := resourceMetric{Timestamp: metav1.NewTime(now), Usage: corev1.ResourceList{corev1.ResourceCPU: quantity.MustParse("2350m"), corev1.ResourceMemory: quantity.MustParse("8Gi")}}
			switch mode {
			case "zero":
				row.Usage[corev1.ResourceCPU] = quantity.MustParse("0")
				row.Usage[corev1.ResourceMemory] = quantity.MustParse("0")
			case "stale":
				row.Timestamp = metav1.NewTime(now.Add(-3 * time.Minute))
			case "future":
				row.Timestamp = metav1.NewTime(now.Add(3 * time.Minute))
			case "before-creation":
				row.Timestamp = metav1.NewTime(now.Add(-2 * time.Hour))
			case "incomplete":
				delete(row.Usage, corev1.ResourceCPU)
			case "negative":
				row.Usage[corev1.ResourceCPU] = quantity.MustParse("-1")
			}
			result := metricSample(target, identity, row, true, "nodes", now)
			valid := mode == "current" || mode == "zero"
			if (result.State == "available") != valid || (result.Usage != nil) != valid {
				t.Fatalf("unexpected sample %+v", result)
			}
		})
	}
}

func TestMetricsErrorsAreUnavailableNotZeroAndHonorCancellation(t *testing.T) {
	for _, code := range []int{403, 404, 503} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(code)
				writeJSON(t, w, fmt.Sprintf(`{"kind":"Status","apiVersion":"v1","status":"Failure","code":%d,"message":"sensitive-server-body"}`, code))
			})
			result, err := client.(metrics.Reader).ResourceUsage(t.Context(), metrics.Query{Resource: "nodes"})
			if err != nil || result.State == "available" || len(result.Items) != 0 || strings.Contains(result.Message, "sensitive") {
				t.Fatalf("unexpected failure: %+v %v", result, err)
			}
		})
	}
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("cancelled metrics request reached API server") })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.(metrics.Reader).ResourceUsage(ctx, metrics.Query{Resource: "nodes"}); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestMetricsResponseIsBounded(t *testing.T) {
	client := testClientFor(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, strings.Repeat(" ", metricsResponseLimit+1))
	})
	result, err := client.(metrics.Reader).ResourceUsage(t.Context(), metrics.Query{Resource: "nodes"})
	if err != nil || result.State != "error" || !strings.Contains(result.Message, "8 MiB") {
		t.Fatalf("%+v %v", result, err)
	}
}
