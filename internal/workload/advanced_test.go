package workload

import (
	"reflect"
	"testing"

	"github.com/cereals-af/cereals-desktop/internal/formvalue"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAdvancedPodAndContainerEditsPreserveUnexposedData(t *testing.T) {
	o := fixture(t)
	snap, err := snapshot(o)
	if err != nil {
		t.Fatal(err)
	}
	options := snap.Form.PodOptions
	if options == nil {
		t.Fatal("missing options")
	}
	advanced := snap.Form.Containers[0].Advanced
	if advanced == nil {
		t.Fatal("missing container options")
	}
	env := advanced.Properties["env"]
	env.Items[0].Properties["value"] = formvalue.Value{Type: "string", Text: "changed"}
	env.Items = env.Items[:1]
	advanced.Properties["env"] = env
	mounts, _ := formvalue.Encode([]interface{}{map[string]interface{}{"name": "data", "mountPath": "/data", "readOnly": true}})
	advanced.Properties["volumeMounts"] = mounts
	init := options.Properties["initContainers"]
	init.Items[0].Properties["image"] = formvalue.Value{Type: "string", Text: "busybox:stable"}
	options.Properties["initContainers"] = init
	if err := updateDeployment(o, snap.Form); err != nil {
		t.Fatal(err)
	}
	containers, _ := containersByName(o, "containers")
	raw := containers["app"]
	if raw["futureField"].(map[string]interface{})["number"] != int64(9007199254740993) {
		t.Fatal("unknown container field changed")
	}
	if raw["env"].([]interface{})[0].(map[string]interface{})["futureField"] != "keep" || len(raw["env"].([]interface{})) != 1 {
		t.Fatal("env removal or field preservation failed")
	}
	user, _, _ := unstructured.NestedInt64(o.Object, "spec", "template", "spec", "securityContext", "runAsUser")
	if user != 9007199254740993 {
		t.Fatal("UID precision lost")
	}
	if !reflect.DeepEqual(raw["volumeMounts"], []interface{}{map[string]interface{}{"name": "data", "mountPath": "/data", "readOnly": true}}) {
		t.Fatal("mount missing")
	}
}

func TestInitContainerNamesCannotConflictWithRegularContainers(t *testing.T) {
	o := fixture(t)
	snap, err := snapshot(o)
	if err != nil {
		t.Fatal(err)
	}
	init := snap.Form.PodOptions.Properties["initContainers"]
	init.Items[0].Properties["name"] = formvalue.Value{Type: "string", Text: "app"}
	snap.Form.PodOptions.Properties["initContainers"] = init
	if err := updateDeployment(o, snap.Form); err == nil {
		t.Fatal("conflicting init name accepted")
	}
}
