package manifest

import (
	"reflect"
	"slices"
	"strings"
)

type patchStep struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// JSON Patch distinguishes explicit null from removing a field. Arrays are replaced
// as a whole, guarded by the resource version, so unrendered item fields survive.
func exactPatch(before, after map[string]interface{}, base Baseline) []patchStep {
	steps := []patchStep{{Op: "test", Path: "/metadata/uid", Value: base.UID}, {Op: "test", Path: "/metadata/resourceVersion", Value: base.ResourceVersion}}
	return appendPatch(steps, "", before, after)
}
func appendPatch(steps []patchStep, path string, before, after map[string]interface{}) []patchStep {
	keys := make([]string, 0, len(before)+len(after))
	for key := range before {
		keys = append(keys, key)
	}
	for key := range after {
		if _, ok := before[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		old, had := before[key]
		next, has := after[key]
		if had && has && reflect.DeepEqual(old, next) {
			continue
		}
		child := path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
		if !has {
			steps = append(steps, patchStep{Op: "remove", Path: child})
			continue
		}
		oldMap, oldOK := old.(map[string]interface{})
		newMap, newOK := next.(map[string]interface{})
		if had && oldOK && newOK {
			steps = appendPatch(steps, child, oldMap, newMap)
			continue
		}
		op := "add"
		if had {
			op = "replace"
		}
		steps = append(steps, patchStep{Op: op, Path: child, Value: next})
	}
	return steps
}
