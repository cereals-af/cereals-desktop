package manifest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var testRef = resource.Ref{ClusterID: "cluster-a", Group: "demo.test", Version: "v1", Resource: "widgets", Namespace: "team", Name: "one"}

const testYAML = "apiVersion: demo.test/v1\nkind: Widget\nmetadata:\n  name: one\n  namespace: team\nspec:\n  sequence: 9007199254740993\n  keep:\n    unknown: original\n  image: old\n"

type testClient struct {
	id     string
	object *unstructured.Unstructured
	writes []Operation
	dry    []bool
	fail   bool
}

func (c *testClient) ConnectionID() string { return c.id }
func (c *testClient) Describe(context.Context, resource.Ref) (resource.Type, error) {
	return resource.Type{Kind: "Widget", Namespaced: true}, nil
}
func (c *testClient) GetObject(context.Context, resource.Ref) (*unstructured.Unstructured, error) {
	return c.object.DeepCopy(), nil
}
func (c *testClient) WriteObject(_ context.Context, op Operation, dry bool) (*unstructured.Unstructured, error) {
	c.writes = append(c.writes, op)
	c.dry = append(c.dry, dry)
	if c.fail {
		return nil, errors.New("conflict")
	}
	return c.object.DeepCopy(), nil
}
func (c *testClient) DeleteObject(context.Context, DeleteRequest, bool) error { return nil }

type testResolver struct{ client *testClient }

func (r *testResolver) ManifestClient(string) (Client, error) { return r.client, nil }

func newTestService(t *testing.T) (*Service, *testClient) {
	t.Helper()
	object, err := parse(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	object.SetUID("uid-one")
	object.SetResourceVersion("12")
	client := &testClient{id: "connection-one", object: object}
	service := NewService(&testResolver{client: client})
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	return service, client
}

func TestParseRejectsAmbiguityAndKeepsIntegerPrecision(t *testing.T) {
	object, err := parse(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	value, _, _ := unstructured.NestedInt64(object.Object, "spec", "sequence")
	if value != 9007199254740993 {
		t.Fatalf("integer rounded: %d", value)
	}
	for _, input := range []string{
		"a: one\na: secret-do-not-log\n", "a: 1\n---\na: 2", "a: &base [1]\nb: *base", "a: 9223372036854775808", "a: .inf", "[a, b]",
	} {
		if _, err := parse(input); err == nil || strings.Contains(err.Error(), "secret-do-not-log") {
			t.Fatalf("unsafe parse result: %v", err)
		}
	}
}

func TestPatchOnlyChangesEditedFieldsAndEnforcesVersion(t *testing.T) {
	s, c := newTestService(t)
	request := Request{Target: testRef, Mode: "patch", YAML: strings.Replace(testYAML, "image: old", "image: new", 1), Baseline: baseline(c.object)}
	preview, err := s.Preview(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	patch := string(c.writes[0].Data)
	if strings.Contains(patch, "unknown") || strings.Contains(patch, "sequence") || !strings.Contains(patch, `"resourceVersion":"12"`) || !strings.Contains(patch, `"uid":"uid-one"`) {
		t.Fatalf("incorrect delta/preconditions: %s", patch)
	}
	if !strings.Contains(preview.Desired, "9007199254740993") {
		t.Fatal("preview rounded CRD integer")
	}
	if _, err := s.Commit(t.Context(), preview.Token); err != nil {
		t.Fatal(err)
	}
	if len(c.dry) != 2 || !c.dry[0] || c.dry[1] {
		t.Fatalf("wrong dry-run sequence: %v", c.dry)
	}
	if _, err := s.Commit(t.Context(), preview.Token); err == nil {
		t.Fatal("replayed a consumed preview")
	}
	request.Baseline.ResourceVersion = "11"
	if _, err := s.Preview(t.Context(), request); err == nil || len(c.writes) != 2 {
		t.Fatal("stale baseline reached dry run")
	}
}

func TestPreviewRejectsScopeChangesAndUntrustedSecretEdits(t *testing.T) {
	s, c := newTestService(t)
	for _, text := range []string{strings.Replace(testYAML, "name: one", "name: other", 1), strings.Replace(testYAML, "namespace: team", "namespace: other", 1), strings.Replace(testYAML, "kind: Widget", "kind: Pod", 1)} {
		if _, err := s.Preview(t.Context(), Request{Target: testRef, Mode: "patch", YAML: text, Baseline: baseline(c.object)}); err == nil {
			t.Fatal("accepted a target change")
		}
	}
	secret := resource.Ref{ClusterID: "cluster-a", Version: "v1", Resource: "secrets", Namespace: "team", Name: "one"}
	c.object.Object["data"] = map[string]interface{}{"token": "secret-value"}
	c.object.SetAnnotations(map[string]string{"original": "secret-annotation"})
	snapshot, err := s.Snapshot(t.Context(), secret, false)
	if err != nil || snapshot.Editable || strings.Contains(snapshot.YAML, "secret-value") || strings.Contains(snapshot.YAML, "secret-annotation") {
		t.Fatal("masked snapshot exposed secrets")
	}
	if _, err := s.Preview(t.Context(), Request{Target: secret, Mode: "patch", YAML: snapshot.YAML, Baseline: snapshot.Baseline}); err == nil {
		t.Fatal("accepted masked Secret as a draft")
	}
}

func TestTicketCannotCrossConnectionExpiryOrFailedCommit(t *testing.T) {
	s, c := newTestService(t)
	request := Request{Target: testRef, Mode: "patch", YAML: testYAML, Baseline: baseline(c.object)}
	preview, err := s.Preview(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	c.id = "new-session"
	if _, err := s.Commit(t.Context(), preview.Token); err == nil {
		t.Fatal("preview crossed a reconnect")
	}
	preview, err = s.Preview(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	ticket := s.tickets[preview.Token]
	ticket.expires = time.Now().Add(-time.Second)
	s.tickets[preview.Token] = ticket
	s.mu.Unlock()
	if _, err := s.Commit(t.Context(), preview.Token); err == nil {
		t.Fatal("expired preview committed")
	}
	preview, err = s.Preview(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	c.fail = true
	if _, err := s.Commit(t.Context(), preview.Token); err == nil {
		t.Fatal("expected failure")
	}
	c.fail = false
	if _, err := s.Commit(t.Context(), preview.Token); err == nil {
		t.Fatal("uncertain failed write retried with old token")
	}
}
