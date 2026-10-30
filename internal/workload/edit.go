package workload

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	resourceRef "github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newDeployment(request Request) (*unstructured.Unstructured, error) {
	return newPodController(request.Target, request.Form, "Deployment", true)
}

func newPodController(target resourceRef.Ref, form Deployment, kind string, replicas bool) (*unstructured.Unstructured, error) {
	if form.Name == "" || form.Namespace == "" || (target.Name != "" && target.Name != form.Name) || (target.Namespace != "" && target.Namespace != form.Namespace) {
		return nil, errors.New("请填写名称和命名空间，且必须与当前操作范围一致")
	}
	if len(form.Containers) == 0 {
		return nil, errors.New("至少需要一个容器")
	}
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": kind,
		"metadata": map[string]interface{}{"name": form.Name, "namespace": form.Namespace},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app.kubernetes.io/name": form.Name}},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{"labels": map[string]interface{}{"app.kubernetes.io/name": form.Name}},
				"spec":     map[string]interface{}{},
			},
		},
	}}
	if replicas {
		if err := setReplicas(object, form.Replicas); err != nil {
			return nil, err
		}
	}
	if err := editPodOptions(object, form.PodOptions); err != nil {
		return nil, err
	}
	err := updateContainers(object, nil, form.Containers)
	return object, err
}

func setReplicas(object *unstructured.Unstructured, replicas int32) error {
	if replicas < 0 {
		return errors.New("副本数不能小于零")
	}
	return unstructured.SetNestedField(object.Object, int64(replicas), "spec", "replicas")
}

func updateDeployment(object *unstructured.Unstructured, form Deployment) error {
	current, err := snapshot(object)
	if err != nil {
		return err
	}
	if form.Name != current.Form.Name || form.Namespace != current.Form.Namespace {
		return errors.New("编辑时不能更改资源名称或命名空间")
	}
	if form.Replicas != current.Form.Replicas {
		if err := setReplicas(object, form.Replicas); err != nil {
			return err
		}
	}
	if err := editPodOptions(object, form.PodOptions); err != nil {
		return err
	}
	return updateContainers(object, current.Form.Containers, form.Containers)
}

func editContainer(raw map[string]interface{}, old, next Container) error {
	if strings.TrimSpace(next.Image) == "" {
		return errors.New("镜像不能为空")
	}
	raw["image"] = next.Image
	if err := editContainerProcess(raw, old, next); err != nil {
		return err
	}
	if !reflect.DeepEqual(old.Ports, next.Ports) {
		if err := editContainerPorts(raw, old.Ports, next.Ports); err != nil {
			return err
		}
	}
	quantities := []struct{ old, next, scope, name string }{
		{old.CPURequest, next.CPURequest, "requests", "cpu"}, {old.CPULimit, next.CPULimit, "limits", "cpu"},
		{old.MemoryRequest, next.MemoryRequest, "requests", "memory"}, {old.MemoryLimit, next.MemoryLimit, "limits", "memory"},
	}
	for _, field := range quantities {
		if field.old == field.next {
			continue
		}
		path := []string{"resources", field.scope, field.name}
		if field.next == "" {
			unstructured.RemoveNestedField(raw, path...)
			continue
		}
		quantity, err := resource.ParseQuantity(field.next)
		if err != nil || quantity.Sign() < 0 {
			return fmt.Errorf("%s %s 需要有效的非负资源量", field.scope, field.name)
		}
		if err := unstructured.SetNestedField(raw, field.next, path...); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(old.Environment, next.Environment) {
		if err := editEnvironment(raw, next.Environment); err != nil {
			return err
		}
	}
	for _, probe := range []struct {
		name      string
		old, next Probe
	}{
		{"readinessProbe", old.Readiness, next.Readiness}, {"livenessProbe", old.Liveness, next.Liveness}, {"startupProbe", old.Startup, next.Startup},
	} {
		if !reflect.DeepEqual(probe.old, probe.next) {
			if err := editProbe(raw, probe.name, probe.old, probe.next); err != nil {
				return err
			}
		}
	}
	if !reflect.DeepEqual(old.Advanced, next.Advanced) {
		return editAdvanced(raw, next.Advanced, containerOptionFields)
	}
	return nil
}

func editEnvironment(raw map[string]interface{}, desired []Environment) error {
	existing, _, err := unstructured.NestedSlice(raw, "env")
	if err != nil {
		return err
	}
	byName := map[string]map[string]interface{}{}
	for _, value := range existing {
		entry, ok := value.(map[string]interface{})
		if !ok {
			return errors.New("环境变量格式无效，请使用 YAML 编辑")
		}
		name, _ := entry["name"].(string)
		if byName[name] != nil {
			return errors.New("存在重复环境变量，请使用 YAML 编辑")
		}
		byName[name] = entry
	}
	result := make([]interface{}, 0, len(desired))
	seen := map[string]bool{}
	for _, env := range desired {
		if env.Name == "" || seen[env.Name] {
			return errors.New("环境变量名称不能为空或重复")
		}
		seen[env.Name] = true
		entry := byName[env.Name]
		if entry == nil {
			entry = map[string]interface{}{"name": env.Name}
		}
		if entry["valueFrom"] == nil {
			if env.Source != "" {
				return errors.New("此表单不能创建 valueFrom 引用")
			}
			entry["value"] = env.Value
		} else if env.Value != "" || env.Source == "" {
			return errors.New("引用环境变量请使用 YAML 编辑")
		}
		result = append(result, entry)
	}
	for name, entry := range byName {
		if entry["valueFrom"] != nil && !seen[name] {
			return errors.New("此表单不能删除引用环境变量")
		}
	}
	if len(result) == 0 {
		delete(raw, "env")
		return nil
	}
	raw["env"] = result
	return nil
}
