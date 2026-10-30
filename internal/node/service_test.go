package node

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var testTarget = resource.Ref{ClusterID: "cluster", Version: "v1", Resource: "nodes", Name: "worker-1"}

type testClient struct {
	mu          sync.Mutex
	node        *unstructured.Unstructured
	pods        map[string]*unstructured.Unstructured
	done        chan struct{}
	waiting     bool
	failures    map[string]bool
	attempts    map[string]int
	dry         int
	listErr     error
	listOptions metav1.ListOptions
}

func testObject(t *testing.T, text string) *unstructured.Unstructured {
	t.Helper()
	object, err := manifest.ParseDraft(text)
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func newTestClient(t *testing.T) *testClient {
	return &testClient{node: testObject(t, `{"apiVersion":"v1","kind":"Node","metadata":{"name":"worker-1","uid":"node-uid","resourceVersion":"1","labels":{"node-role.kubernetes.io/worker":""}},"spec":{"unschedulable":false,"unknown":9007199254740993},"status":{"capacity":{"cpu":"8","memory":"32Gi"},"allocatable":{"cpu":"7800m","memory":"30Gi"}}}`), pods: map[string]*unstructured.Unstructured{}, done: make(chan struct{}), failures: map[string]bool{}, attempts: map[string]int{}}
}

func (c *testClient) ConnectionID() string { return "connection" }

func (c *testClient) SessionDone() <-chan struct{} { return c.done }

func (c *testClient) GetObject(context.Context, resource.Ref) (*unstructured.Unstructured, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.node.DeepCopy(), nil
}

func (c *testClient) ListObjects(_ context.Context, ref resource.Ref, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listOptions = options
	if c.listErr != nil {
		return nil, c.listErr
	}
	page := &unstructured.UnstructuredList{}
	for _, pod := range c.pods {
		if ref.Namespace == "" || pod.GetNamespace() == ref.Namespace {
			page.Items = append(page.Items, *pod.DeepCopy())
		}
	}
	return page, nil
}

func (c *testClient) ReadDrainPod(_ context.Context, ref resource.Ref) (*unstructured.Unstructured, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if pod := c.pods[ref.Name]; pod != nil {
		return pod.DeepCopy(), nil
	}
	return nil, nil
}

func (c *testClient) EvictPod(_ context.Context, request Eviction) (EvictionResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pod := c.pods[request.Target.Name]
	if pod == nil {
		return EvictionResult{}, errors.New("missing")
	}
	if formedit.Baseline(pod) != request.Baseline {
		return EvictionResult{}, errors.New("wrong preconditions")
	}
	if request.DryRun {
		c.dry++
		return EvictionResult{Waiting: c.waiting}, nil
	}
	c.attempts[request.Target.Name]++
	if c.failures[request.Target.Name] {
		return EvictionResult{}, errors.New("transport result unknown")
	}
	if c.waiting {
		return EvictionResult{Waiting: true}, nil
	}
	delete(c.pods, request.Target.Name)
	return EvictionResult{}, nil
}

type testResolver struct{ client *testClient }

func (r testResolver) NodeClient(string) (Client, error) { return r.client, nil }

type testWriter struct {
	client  *testClient
	mu      sync.Mutex
	request manifest.Request
	commits int
	hook    func()
}

func (w *testWriter) Preview(_ context.Context, r manifest.Request) (manifest.Preview, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.request = r
	return manifest.Preview{Token: "cordon", Target: r.Target, Before: "before", After: r.YAML}, nil
}

func (w *testWriter) Commit(context.Context, string) (manifest.Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client.mu.Lock()
	if formedit.Baseline(w.client.node) != w.request.Baseline {
		w.client.mu.Unlock()
		return manifest.Result{}, errors.New("node changed")
	}
	w.commits++
	_ = unstructured.SetNestedField(w.client.node.Object, true, "spec", "unschedulable")
	w.client.node.SetResourceVersion("2")
	w.client.mu.Unlock()
	if w.hook != nil {
		w.hook()
	}
	return manifest.Result{Target: testTarget}, nil
}

func (w *testWriter) Discard(string) {}

func setup(t *testing.T) (*Service, *testClient, *testWriter) {
	t.Helper()
	c := newTestClient(t)
	w := &testWriter{client: c}
	s := NewService(t.Context(), testResolver{c}, w)
	s.interval = time.Millisecond
	t.Cleanup(func() { _ = s.ServiceShutdown() })
	return s, c, w
}

func addPod(t *testing.T, c *testClient, name string) {
	t.Helper()
	c.pods[name] = testObject(t, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"`+name+`","namespace":"team","uid":"`+name+`-uid","resourceVersion":"10","ownerReferences":[{"apiVersion":"apps/v1","kind":"ReplicaSet","name":"app","uid":"owner","controller":true}]},"spec":{"nodeName":"worker-1","containers":[{"name":"main","image":"example"}]},"status":{"phase":"Running"}}`)
}

func prepare(t *testing.T, s *Service, c *testClient) Plan {
	t.Helper()
	plan, err := s.PreviewDrain(t.Context(), DrainRequest{Target: testTarget, Baseline: formedit.Baseline(c.node), Options: DrainOptions{TimeoutSeconds: 30, IgnoreDaemonSets: true}})
	if err != nil || plan.Token == "" {
		t.Fatalf("preview: %+v %v", plan, err)
	}
	return plan
}

func awaitDone(t *testing.T, s *Service, id string) Progress {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		progress, err := s.PollDrain(id)
		if err != nil {
			t.Fatal(err)
		}
		if progress.Done {
			return progress
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("task did not end")
	return Progress{}
}

func start(t *testing.T, s *Service, plan Plan) string {
	t.Helper()
	id, err := s.StartDrain(t.Context(), plan.Token, testTarget.Name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCordonPreservesFullObjectAndRejectsStaleBaseline(t *testing.T) {
	s, c, w := setup(t)
	before := c.node.DeepCopy()
	request := CordonRequest{Target: testTarget, Baseline: formedit.Baseline(before), Unschedulable: true}
	if _, err := s.PreviewCordon(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	desired := testObject(t, w.request.YAML)
	value, _, _ := unstructured.NestedInt64(desired.Object, "spec", "unknown")
	if value != 9007199254740993 || !reflect.DeepEqual(before, c.node) {
		t.Fatal("preview changed source or lost precision")
	}
	request.Baseline.ResourceVersion = "stale"
	if _, err := s.PreviewCordon(t.Context(), request); err == nil {
		t.Fatal("stale node accepted")
	}
}

func TestDrainPoliciesAreExplicit(t *testing.T) {
	base := Pod{Category: "unmanaged", Phase: "Running", LocalData: []string{"cache (emptyDir)"}}
	for _, options := range []DrainOptions{{}, {AllowUnmanaged: true}, {AllowLocalData: true}} {
		if classify(base, options).State != "blocked" {
			t.Fatal("unsafe default")
		}
	}
	if classify(base, DrainOptions{AllowUnmanaged: true, AllowLocalData: true}).State != "pending" {
		t.Fatal("explicit policy ignored")
	}
	base.Category = "daemonset"
	if classify(base, DrainOptions{}).State != "blocked" || classify(base, DrainOptions{IgnoreDaemonSets: true}).State != "skipped" {
		t.Fatal("daemonset policy")
	}
	base.Category = "static"
	if classify(base, DrainOptions{}).State != "skipped" {
		t.Fatal("static mirror evicted")
	}
}

func TestDrainLifecycleAndPartialFailures(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{true: "partial", false: "complete"}[failure], func(t *testing.T) {
			s, c, _ := setup(t)
			addPod(t, c, "one")
			addPod(t, c, "two")
			c.failures["one"] = failure
			plan := prepare(t, s, c)
			if len(c.pods) != 2 || c.dry != 2 || plan.Cordon.Token != "" {
				t.Fatal("dry run changed state or exposed cordon token")
			}
			id := start(t, s, plan)
			if _, err := s.StartDrain(t.Context(), plan.Token, testTarget.Name); err == nil {
				t.Fatal("plan replay")
			}
			result := awaitDone(t, s, id)
			expected := "completed"
			if failure {
				expected = "failed"
			}
			if result.State != expected {
				t.Fatalf("result %+v", result)
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.attempts["one"] != 1 || c.attempts["two"] != 1 {
				t.Fatal("ambiguous request replayed")
			}
			if len(c.pods) > 0 && !failure {
				t.Fatal("pods retained")
			}
		})
	}
}

func TestDrainingRejectsChangedInventoryBeforeCordon(t *testing.T) {
	s, c, w := setup(t)
	addPod(t, c, "one")
	plan := prepare(t, s, c)
	addPod(t, c, "new")
	result := awaitDone(t, s, start(t, s, plan))
	if result.State != "failed" || !strings.Contains(result.Message, "新 Pod") || w.commits != 0 {
		t.Fatalf("unreviewed pod accepted: %+v", result)
	}
}

func TestSameNameReplacementIsNeverEvicted(t *testing.T) {
	s, c, w := setup(t)
	addPod(t, c, "one")
	plan := prepare(t, s, c)
	w.hook = func() { c.mu.Lock(); defer c.mu.Unlock(); c.pods["one"].SetUID("replacement") }
	result := awaitDone(t, s, start(t, s, plan))
	if result.State != "failed" {
		t.Fatalf("remaining pod reported drained: %+v", result)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attempts["one"] != 0 {
		t.Fatal("replacement evicted")
	}
}

func TestPDBWaitCanBeCancelledOrDisconnected(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		t.Run(map[bool]string{true: "disconnect", false: "cancel"}[disconnect], func(t *testing.T) {
			s, c, _ := setup(t)
			addPod(t, c, "one")
			c.waiting = true
			plan := prepare(t, s, c)
			id := start(t, s, plan)
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				c.mu.Lock()
				attempts := c.attempts["one"]
				c.mu.Unlock()
				if attempts > 0 {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if disconnect {
				close(c.done)
			} else {
				s.CancelDrain(id)
			}
			result := awaitDone(t, s, id)
			if result.State != "cancelled" {
				t.Fatalf("cancel: %+v", result)
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if len(c.pods) != 1 {
				t.Fatal("PDB bypassed")
			}
		})
	}
}

func TestNodePodQueryAndPartialPermission(t *testing.T) {
	s, c, _ := setup(t)
	addPod(t, c, "one")
	page, err := s.Pods(t.Context(), testTarget, "team", "")
	if err != nil || len(page.Pods) != 1 || c.listOptions.FieldSelector != "spec.nodeName=worker-1" || c.listOptions.Limit != 100 {
		t.Fatalf("node selector missing: %+v %v", page, err)
	}
	c.listErr = errors.New("forbidden")
	if _, err := s.Inspect(t.Context(), testTarget); err != nil {
		t.Fatal("pod permission hid node")
	}
	if _, err := s.PreviewDrain(t.Context(), DrainRequest{Target: testTarget, Options: DrainOptions{TimeoutSeconds: 30}}); err == nil {
		t.Fatal("partial inventory accepted")
	}
}

func TestAbandonedLeaseAndExpiredPlansAreReclaimed(t *testing.T) {
	s, c, _ := setup(t)
	addPod(t, c, "one")
	c.waiting = true
	plan := prepare(t, s, c)
	id := start(t, s, plan)
	s.mu.Lock()
	s.tasks[id].touched = time.Now().Add(-2 * taskLease)
	s.mu.Unlock()
	s.expire(time.Now())
	result := awaitDone(t, s, id)
	if result.State != "cancelled" {
		t.Fatalf("abandoned task still active: %+v", result)
	}
	s.CloseDrain(id)
	if _, err := s.PollDrain(id); err == nil {
		t.Fatal("closed task retained")
	}
	snapshot, err := s.Inspect(t.Context(), testTarget)
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.PreviewDrain(t.Context(), DrainRequest{Target: testTarget, Baseline: snapshot.Baseline, Options: DrainOptions{TimeoutSeconds: 30}})
	if err != nil {
		t.Fatal(err)
	}
	s.expire(time.Now().Add(planTTL + time.Second))
	if _, err := s.StartDrain(t.Context(), next.Token, testTarget.Name); err == nil {
		t.Fatal("expired plan started")
	}
}

func TestDeadlineReportsUnfinishedWork(t *testing.T) {
	s, c, w := setup(t)
	addPod(t, c, "one")
	c.waiting = true
	plan := prepare(t, s, c)
	// Shorten only this internal test deadline; public input remains 30–3600 seconds.
	s.mu.Lock()
	pending := s.plans[plan.Token]
	pending.request.Options.TimeoutSeconds = 0
	s.plans[plan.Token] = pending
	s.mu.Unlock()
	result := awaitDone(t, s, start(t, s, plan))
	if result.State != "timeout" || w.commits != 0 {
		t.Fatalf("deadline not reported: %+v", result)
	}
}

func TestPodStatusChangesDoNotInvalidateReviewedSpec(t *testing.T) {
	s, c, _ := setup(t)
	addPod(t, c, "one")
	plan := prepare(t, s, c)
	c.pods["one"].SetResourceVersion("11")
	_ = unstructured.SetNestedField(c.pods["one"].Object, "Succeeded", "status", "phase")
	result := awaitDone(t, s, start(t, s, plan))
	if result.State != "completed" {
		t.Fatalf("normal status update invalidated plan: %+v", result)
	}
}

func TestPodOutsideNodeCannotEnterInventory(t *testing.T) {
	s, c, _ := setup(t)
	addPod(t, c, "one")
	_ = unstructured.SetNestedField(c.pods["one"].Object, "other-node", "spec", "nodeName")
	if _, err := s.Pods(t.Context(), testTarget, "", ""); err == nil {
		t.Fatal("unrelated node Pod accepted")
	}
}
