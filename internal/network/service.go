package network

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Service struct{ editor *formedit.Editor }

func NewService(resolver manifest.Resolver, writer formedit.Previewer) *Service {
	return &Service{editor: formedit.New(resolver, writer)}
}

func (s *Service) Snapshot(ctx context.Context, target resource.Ref) (Snapshot, error) {
	if _, err := formedit.Target(target, "services", target.Name, target.Namespace); err != nil {
		return Snapshot{}, err
	}
	object, err := s.editor.Read(ctx, target)
	if err != nil {
		return Snapshot{}, err
	}
	return serviceSnapshot(object)
}

func (s *Service) Preview(ctx context.Context, request Request) (manifest.Preview, error) {
	target, err := formedit.Target(request.Target, "services", request.Form.Name, request.Form.Namespace)
	if err != nil {
		return manifest.Preview{}, err
	}
	return s.editor.Preview(ctx, target, request.Baseline, request.Mode, "Service", func(object *unstructured.Unstructured) error {
		return editService(object, request.Form, request.Mode == "create")
	})
}

func serviceSnapshot(object *unstructured.Unstructured) (Snapshot, error) {
	var service corev1.Service
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &service); err != nil {
		return Snapshot{}, errors.New("Service 字段格式无效，请使用 YAML 编辑")
	}
	spec := service.Spec
	form := ServiceForm{Name: service.Name, Namespace: service.Namespace, Type: string(spec.Type), Headless: spec.ClusterIP == "None", ExternalName: spec.ExternalName,
		Selector: []Selector{}, Ports: []Port{}, ExternalTrafficPolicy: string(spec.ExternalTrafficPolicy), SessionAffinity: string(spec.SessionAffinity), PublishNotReadyAddresses: spec.PublishNotReadyAddresses}
	if form.Type == "" {
		form.Type = "ClusterIP"
	}
	if form.SessionAffinity == "" {
		form.SessionAffinity = "None"
	}
	for key, value := range spec.Selector {
		form.Selector = append(form.Selector, Selector{Key: key, Value: value})
	}
	slices.SortFunc(form.Selector, func(a, b Selector) int { return strings.Compare(a.Key, b.Key) })
	for index, port := range spec.Ports {
		form.Ports = append(form.Ports, Port{OriginalKey: portKey(port.Name, int64(port.Port), string(port.Protocol)), Name: port.Name, Protocol: string(port.Protocol), Port: port.Port, TargetPort: port.TargetPort.String(), NodePort: port.NodePort})
		if port.AppProtocol != nil {
			form.Ports[index].AppProtocol = *port.AppProtocol
		}
	}
	result := Snapshot{Form: form, Baseline: formedit.Baseline(object), ClusterIPs: slices.Clone(spec.ClusterIPs), IPFamilies: []string{}, LoadBalancerAddresses: []string{}}
	for _, family := range spec.IPFamilies {
		result.IPFamilies = append(result.IPFamilies, string(family))
	}
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			result.LoadBalancerAddresses = append(result.LoadBalancerAddresses, ingress.IP)
		}
		if ingress.Hostname != "" {
			result.LoadBalancerAddresses = append(result.LoadBalancerAddresses, ingress.Hostname)
		}
	}
	return result, nil
}
