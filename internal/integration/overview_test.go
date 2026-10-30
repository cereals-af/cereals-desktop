//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/overview"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	corev1 "k8s.io/api/core/v1"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type overviewResolver struct{ client overview.Client }

func (r overviewResolver) OverviewClient(string) (overview.Client, error) { return r.client, nil }

func TestAPIServerClusterOverview(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client, err := coreclient.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"overview-one", "overview-two"} {
		if _, err := client.Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Secrets(name).Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Annotations: map[string]string{"private": "never-return-this"}}, Data: map[string][]byte{"key": []byte("never-return-this")}}, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	node, err := client.Nodes().Create(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "overview-node"}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	node.Status.Capacity = corev1.ResourceList{corev1.ResourceCPU: quantity.MustParse("4"), corev1.ResourceMemory: quantity.MustParse("8Gi"), corev1.ResourcePods: quantity.MustParse("110")}
	node.Status.Allocatable = corev1.ResourceList{corev1.ResourceCPU: quantity.MustParse("3500m"), corev1.ResourceMemory: quantity.MustParse("7Gi")}
	node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}
	if _, err := client.Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	connection, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	reader := connection.(overview.Client)
	count, err := reader.CountResources(ctx, resource.Type{Version: "v1", Resource: "secrets", Namespaced: true})
	if err != nil || count != 2 {
		t.Fatalf("cross namespace count: %d %v", count, err)
	}
	snapshot, err := overview.NewService(overviewResolver{reader}).Snapshot(ctx, "test-only")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Nodes == nil || snapshot.Nodes.Count != 1 || snapshot.Nodes.Ready != 1 || snapshot.Nodes.Capacity["memory"] != "8Gi" || snapshot.Nodes.Allocatable["cpu"] != "3.500" {
		t.Fatalf("incorrect capacity: %+v", snapshot.Nodes)
	}
	if len(snapshot.Resources) < 10 {
		t.Fatal("catalog was not counted")
	}
	data, err := json.Marshal(snapshot)
	if err != nil || strings.Contains(string(data), "never-return-this") {
		t.Fatal("overview exposed resource content")
	}
	restricted := rest.CopyConfig(config)
	restricted.Impersonate = rest.ImpersonationConfig{UserName: "overview-reader"}
	denied, err := (kube.Factory{}).New(ctx, restricted)
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Close()
	if _, err := denied.(overview.Client).CountResources(ctx, resource.Type{Version: "v1", Resource: "secrets"}); err == nil {
		t.Fatal("forbidden list was counted")
	}
	if _, err := denied.(overview.Client).OverviewNodes(ctx); err == nil {
		t.Fatal("forbidden node list succeeded")
	}
}
