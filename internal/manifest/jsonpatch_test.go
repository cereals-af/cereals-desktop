package manifest

import (
	"encoding/json"
	"reflect"
	"testing"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
)

func TestExactPatchDistinguishesNullAndEscapesPropertyNames(t *testing.T) {
	before := map[string]interface{}{"metadata": map[string]interface{}{"uid": "one", "resourceVersion": "10"}, "spec": map[string]interface{}{"a/b~c": "before", "removed": "old", "retained": int64(9007199254740993)}}
	after := map[string]interface{}{"metadata": map[string]interface{}{"uid": "one", "resourceVersion": "10"}, "spec": map[string]interface{}{"a/b~c": nil, "retained": int64(9007199254740993), "added": true}}
	data, err := json.Marshal(exactPatch(before, after, Baseline{UID: "one", ResourceVersion: "10"}))
	if err != nil {
		t.Fatal(err)
	}
	patch, err := jsonpatch.DecodePatch(data)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := json.Marshal(before)
	result, err := patch.Apply(source)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := parse(string(result))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Object, after) {
		t.Fatalf("unexpected patch result %s", result)
	}
	before["metadata"].(map[string]interface{})["uid"] = "replacement"
	source, _ = json.Marshal(before)
	if _, err := patch.Apply(source); err == nil {
		t.Fatal("patch crossed recreated identity")
	}
}
