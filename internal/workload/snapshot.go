package workload

import (
	"fmt"
	"slices"

	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/workloadstate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func snapshot(object *unstructured.Unstructured) (Snapshot, error) {
	var deployment appsv1.Deployment
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &deployment); err != nil {
		return Snapshot{}, fmt.Errorf("读取 Deployment 字段: %w", err)
	}
	form := Deployment{Name: deployment.Name, Namespace: deployment.Namespace, Replicas: 1, Containers: []Container{}}
	if deployment.Spec.Replicas != nil {
		form.Replicas = *deployment.Spec.Replicas
	}
	options, err := podOptions(object)
	form.PodOptions = options
	if err != nil {
		return Snapshot{}, err
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		form.Containers = append(form.Containers, containerForm(container))
	}
	if err := attachContainerOptions(object, form.Containers); err != nil {
		return Snapshot{}, err
	}
	status := deploymentStatus(deployment, form.Replicas)
	status.State, status.Detail, _ = workloadstate.Classify(*object)
	return Snapshot{
		Form:     form,
		Baseline: manifest.Baseline{UID: string(deployment.UID), ResourceVersion: deployment.ResourceVersion},
		Status:   status,
	}, nil
}

func containerForm(container corev1.Container) Container {
	form := Container{OriginalName: container.Name, Name: container.Name, Image: container.Image, Environment: []Environment{},
		Command: slices.Clone(container.Command), Args: slices.Clone(container.Args), Ports: []ContainerPort{},
		Readiness: probeForm(container.ReadinessProbe), Liveness: probeForm(container.LivenessProbe), Startup: probeForm(container.StartupProbe)}
	for _, port := range container.Ports {
		protocol := string(port.Protocol)
		if protocol == "" {
			protocol = "TCP"
		}
		form.Ports = append(form.Ports, ContainerPort{OriginalKey: containerPortKey(port.Name, port.ContainerPort, protocol), Name: port.Name,
			Protocol: protocol, ContainerPort: port.ContainerPort, HostPort: port.HostPort, HostIP: port.HostIP})
	}
	form.CPURequest = quantity(container.Resources.Requests, corev1.ResourceCPU)
	form.CPULimit = quantity(container.Resources.Limits, corev1.ResourceCPU)
	form.MemoryRequest = quantity(container.Resources.Requests, corev1.ResourceMemory)
	form.MemoryLimit = quantity(container.Resources.Limits, corev1.ResourceMemory)
	for _, env := range container.Env {
		source := ""
		if env.ValueFrom != nil {
			source = "valueFrom（可在环境变量引用中查看和编辑）"
		}
		form.Environment = append(form.Environment, Environment{Name: env.Name, Value: env.Value, Source: source})
	}
	return form
}

func quantity(list corev1.ResourceList, name corev1.ResourceName) string {
	if value, ok := list[name]; ok {
		return value.String()
	}
	return ""
}

func probeForm(probe *corev1.Probe) Probe {
	if probe == nil {
		return Probe{Kind: "none", Scheme: "HTTP", Timeout: 1, Period: 10, SuccessThreshold: 1, FailureThreshold: 3, Command: []string{}}
	}
	form := Probe{Kind: "keep", Scheme: "HTTP", Command: []string{}, InitialDelay: probe.InitialDelaySeconds, Timeout: probe.TimeoutSeconds,
		Period: probe.PeriodSeconds, SuccessThreshold: probe.SuccessThreshold, FailureThreshold: probe.FailureThreshold}
	switch {
	case probe.HTTPGet != nil:
		form.Kind, form.Path, form.Port, form.Scheme = "http", probe.HTTPGet.Path, probe.HTTPGet.Port.String(), string(probe.HTTPGet.Scheme)
	case probe.TCPSocket != nil:
		form.Kind, form.Port = "tcp", probe.TCPSocket.Port.String()
	case probe.Exec != nil:
		form.Kind, form.Command = "exec", probe.Exec.Command
	}
	return form
}

func deploymentStatus(d appsv1.Deployment, desired int32) Status {
	status := Status{Phase: "发布中", Strategy: string(d.Spec.Strategy.Type), Paused: d.Spec.Paused,
		Observed: d.Status.ObservedGeneration >= d.Generation, Replicas: d.Status.Replicas,
		Updated: d.Status.UpdatedReplicas, Ready: d.Status.ReadyReplicas, Available: d.Status.AvailableReplicas, Conditions: []Condition{}}
	for _, condition := range d.Status.Conditions {
		status.Conditions = append(status.Conditions, Condition{Type: string(condition.Type), Status: string(condition.Status), Reason: condition.Reason, Message: condition.Message})
	}
	switch {
	case d.DeletionTimestamp != nil:
		status.Phase = "删除中"
	case d.Spec.Paused:
		status.Phase = "已暂停"
	case !status.Observed:
		status.Phase = "等待控制器处理"
	case status.Updated == desired && status.Replicas == desired && status.Available == desired && status.Ready == desired:
		status.Phase = "已就绪"
	default:
		for _, condition := range d.Status.Conditions {
			if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded" {
				status.Phase = "发布超时"
			}
		}
	}
	return status
}
