package network

import (
	"errors"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

func editService(object *unstructured.Unstructured, form ServiceForm, creating bool) error {
	if !slices.Contains([]string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"}, form.Type) {
		return errors.New("不支持的 Service 类型")
	}
	previous, err := serviceSnapshot(object)
	if err != nil {
		return err
	}
	spec, _, err := unstructured.NestedMap(object.Object, "spec")
	if err != nil {
		return errors.New("Service spec 格式无效")
	}
	if spec == nil {
		spec = map[string]interface{}{}
	}
	if form.Type == "ExternalName" {
		if len(validation.IsDNS1123Subdomain(strings.TrimSuffix(form.ExternalName, "."))) != 0 {
			return errors.New("ExternalName 需要有效的 DNS 名称")
		}
		if previous.Form.Type != "ExternalName" || creating {
			clearClusterFields(spec)
		}
		spec["type"], spec["externalName"] = form.Type, form.ExternalName
	} else {
		if err := editClusterService(spec, previous.Form, form, creating); err != nil {
			return err
		}
	}
	return unstructured.SetNestedMap(object.Object, spec, "spec")
}

func editClusterService(spec map[string]interface{}, old, form ServiceForm, creating bool) error {
	if form.Headless && form.Type != "ClusterIP" {
		return errors.New("无头服务必须使用 ClusterIP 类型")
	}
	if !creating && old.Type != "ExternalName" && form.Headless != old.Headless {
		return errors.New("现有 Service 的 ClusterIP/无头模式不能通过修改切换，请创建新服务")
	}
	if form.Headless && (creating || old.Type == "ExternalName") {
		spec["clusterIP"] = "None"
	}
	selector, err := selectorMap(form.Selector)
	if err != nil {
		return err
	}
	if len(selector) == 0 {
		delete(spec, "selector")
	} else {
		spec["selector"] = selector
	}
	if err := editPorts(spec, form, creating); err != nil {
		return err
	}
	spec["type"] = form.Type
	delete(spec, "externalName")
	if !slices.Contains([]string{"None", "ClientIP"}, form.SessionAffinity) {
		return errors.New("请选择有效的会话保持策略")
	}
	spec["sessionAffinity"] = form.SessionAffinity
	if form.SessionAffinity == "None" && old.SessionAffinity != "None" {
		delete(spec, "sessionAffinityConfig")
	}
	if form.PublishNotReadyAddresses != old.PublishNotReadyAddresses {
		spec["publishNotReadyAddresses"] = form.PublishNotReadyAddresses
	}
	if form.Type == "NodePort" || form.Type == "LoadBalancer" {
		policy := form.ExternalTrafficPolicy
		if policy == "" {
			policy = "Cluster"
		}
		if !slices.Contains([]string{"Cluster", "Local"}, policy) {
			return errors.New("请选择有效的外部流量策略")
		}
		spec["externalTrafficPolicy"] = policy
		if policy != "Local" {
			delete(spec, "healthCheckNodePort")
		}
	} else {
		delete(spec, "externalTrafficPolicy")
		delete(spec, "healthCheckNodePort")
	}
	if form.Type != "LoadBalancer" && old.Type == "LoadBalancer" {
		clearLoadBalancerFields(spec)
	}
	return nil
}

func selectorMap(entries []Selector) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(entries))
	for _, entry := range entries {
		if len(validation.IsQualifiedName(entry.Key)) != 0 || len(validation.IsValidLabelValue(entry.Value)) != 0 {
			return nil, errors.New("Service 选择器需要有效的标签键和值")
		}
		if _, exists := result[entry.Key]; exists {
			return nil, errors.New("Service 选择器键不能重复")
		}
		result[entry.Key] = entry.Value
	}
	return result, nil
}

func clearLoadBalancerFields(spec map[string]interface{}) {
	for _, field := range []string{"healthCheckNodePort", "allocateLoadBalancerNodePorts", "loadBalancerClass", "loadBalancerIP", "loadBalancerSourceRanges"} {
		delete(spec, field)
	}
}

func clearClusterFields(spec map[string]interface{}) {
	for _, field := range []string{"clusterIP", "clusterIPs", "ipFamilies", "ipFamilyPolicy", "selector", "ports", "externalIPs", "internalTrafficPolicy", "externalTrafficPolicy", "trafficDistribution", "sessionAffinity", "sessionAffinityConfig", "publishNotReadyAddresses"} {
		delete(spec, field)
	}
	clearLoadBalancerFields(spec)
}
