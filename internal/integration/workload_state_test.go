//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/observe"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

func TestAPIServerWorkloadHealthAndChildWatch(t *testing.T) {
	config := controlPlane(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	api, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "health"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "test.invalid/image"}}}}
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "health"}}
	deployment, err := api.AppsV1().Deployments("default").Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "health"}, Spec: appsv1.DeploymentSpec{Replicas: ptr.To(int32(2)), Selector: selector, Template: template}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	replica, err := api.AppsV1().ReplicaSets("default").Create(ctx, &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "health-rs", OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name, UID: deployment.UID, Controller: ptr.To(true)}}}, Spec: appsv1.ReplicaSetSpec{Replicas: ptr.To(int32(2)), Selector: selector, Template: template}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod, err := api.CoreV1().Pods("default").Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "health-pod", OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: replica.Name, UID: replica.UID, Controller: ptr.To(true)}}}, Spec: template.Spec}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := (kube.Factory{}).New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	query := resource.Query{Group: "apps", Version: "v1", Resource: "deployments", Namespace: "default"}
	assertState := func(want string) resource.Page {
		t.Helper()
		page, err := conn.List(ctx, query)
		if err != nil || len(page.Items) != 1 || page.Items[0].Status != want {
			t.Fatalf("state expected %s: %+v %v", want, page, err)
		}
		return page
	}
	first := assertState(workloadstate.Processing)
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	changes := make(chan observe.Change, 32)
	done := make(chan error, 1)
	go func() {
		done <- conn.(observe.Client).Observe(watchCtx, query, first.ResourceVersion, func(change observe.Change) {
			select {
			case changes <- change:
			default:
			}
		})
	}()
	select {
	case <-changes:
	case <-ctx.Done():
		t.Fatal("watch never started")
	}
	pod.Status.Phase = corev1.PodRunning
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", Image: "test.invalid/image", ImageID: "test", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	pod, err = api.CoreV1().Pods("default").UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertState(workloadstate.Ready)
	pod.Status.ContainerStatuses[0].Ready = false
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}
	if _, err = api.CoreV1().Pods("default").UpdateStatus(ctx, pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	assertState(workloadstate.Error)
	dirty := false
	for !dirty {
		select {
		case change := <-changes:
			dirty = change.Dirty
		case <-ctx.Done():
			t.Fatal("Pod changes did not invalidate workload list")
		}
	}
	stopWatch()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("child watches survived cancellation")
	}
}
