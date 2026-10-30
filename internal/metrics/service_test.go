package metrics

import (
	"context"
	"errors"
	"testing"
)

type testReader struct{}

func (testReader) ResourceUsage(ctx context.Context, query Query) (Snapshot, error) {
	if _, ok := ctx.Deadline(); !ok {
		return Snapshot{}, errors.New("missing deadline")
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{State: "available"}, nil
}

type testResolver struct{ clusterID string }

func (r *testResolver) MetricsReader(id string) (Reader, error) {
	r.clusterID = id
	return testReader{}, nil
}

func TestServiceValidatesScopeAndPreservesCancellation(t *testing.T) {
	r := &testResolver{}
	s := NewService(r)
	valid := Query{ClusterID: "one", Resource: "pods", Namespace: "team", Targets: []Target{{Name: "pod", Namespace: "team", UID: "uid"}}}
	if _, err := s.Snapshot(t.Context(), valid); err != nil || r.clusterID != "one" {
		t.Fatalf("%v %+v", err, r)
	}
	invalid := valid
	invalid.Targets = []Target{{Name: "pod", Namespace: "other", UID: "uid"}}
	if _, err := s.Snapshot(t.Context(), invalid); err == nil {
		t.Fatal("cross-namespace query accepted")
	}
	invalid = valid
	invalid.Resource = "secrets"
	if _, err := s.Snapshot(t.Context(), invalid); err == nil {
		t.Fatal("unsupported resource accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.Snapshot(ctx, valid); err == nil {
		t.Fatal("cancelled request accepted")
	}
}
