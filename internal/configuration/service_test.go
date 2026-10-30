package configuration

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func configFixture(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	object := &unstructured.Unstructured{}
	if err := object.UnmarshalJSON([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"config","namespace":"default","uid":"uid","resourceVersion":"10","annotations":{"other":"keep"}},"data":{"empty":"","number":"9007199254740993","yaml":"enabled: true\n  中文\n"},"binaryData":{"payload":"AAEC/w=="},"futureField":9007199254740993}`)); err != nil {
		t.Fatal(err)
	}
	return object
}

func TestConfigMapPreservesTextBinaryAndMetadata(t *testing.T) {
	object := configFixture(t)
	before := object.DeepCopy()
	form, err := configMapForm(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := editConfigMap(object, form); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, object) {
		t.Fatal("unchanged config was normalized")
	}
	form.Data = append(form.Data, Entry{Key: "new.conf", Value: "  before\n中文\nafter  "})
	if err := editConfigMap(object, form); err != nil {
		t.Fatal(err)
	}
	_ = unstructured.SetNestedField(before.Object, "  before\n中文\nafter  ", "data", "new.conf")
	if !reflect.DeepEqual(before, object) {
		t.Fatal("config edit lost fields or text formatting")
	}
}

func TestConfigMapImmutableAndValidation(t *testing.T) {
	cases := map[string]func(*ConfigMap){
		"duplicate key":          func(f *ConfigMap) { f.Data = append(f.Data, f.Data[0]) },
		"overlapping binary key": func(f *ConfigMap) { f.BinaryData[0].Key = f.Data[0].Key },
		"invalid key":            func(f *ConfigMap) { f.Data[0].Key = "invalid/key" },
		"invalid base64":         func(f *ConfigMap) { f.BinaryData[0].Value = "not base64" },
		"too large":              func(f *ConfigMap) { f.Data[0].Value = strings.Repeat("x", 1<<20) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			object := configFixture(t)
			form, _ := configMapForm(object)
			change(&form)
			if err := editConfigMap(object, form); err == nil {
				t.Fatal("invalid ConfigMap accepted")
			}
		})
	}
	object := configFixture(t)
	form, _ := configMapForm(object)
	form.Immutable = true
	if err := editConfigMap(object, form); err != nil {
		t.Fatal(err)
	}
	if err := editConfigMap(object, form); err != nil {
		t.Fatal("immutable unchanged form rejected")
	}
	form.Data[0].Value = "changed"
	if err := editConfigMap(object, form); err == nil {
		t.Fatal("immutable data edited")
	}
	form, _ = configMapForm(object)
	form.Immutable = false
	if err := editConfigMap(object, form); err == nil {
		t.Fatal("immutable flag reverted")
	}
}
