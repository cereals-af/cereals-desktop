package kube

import (
	"strings"
	"testing"
)

func TestCustomTableKeepsNumericTextAndIdentity(t *testing.T) {
	data := []byte(`{"kind":"Table","metadata":{"continue":"next","resourceVersion":"15"},"columnDefinitions":[{"name":"Sequence","type":"integer"}],"rows":[{"cells":[9007199254740993],"object":{"metadata":{"uid":"one","name":"item","namespace":"team"}}}]}`)
	page, err := decodeCustomTable(data)
	if err != nil {
		t.Fatal(err)
	}
	if page.Rows[0].Cells[0] != "9007199254740993" || page.Rows[0].UID != "one" || page.Continue != "next" {
		t.Fatalf("lossy table: %+v", page)
	}
	if _, err := decodeCustomTable([]byte(strings.Replace(string(data), `"uid":"one",`, "", 1))); err == nil {
		t.Fatal("missing identity accepted")
	}
	if _, err := decodeCustomTable([]byte(`{"kind":"WidgetList"}`)); err == nil {
		t.Fatal("unexpected negotiation accepted")
	}
}
