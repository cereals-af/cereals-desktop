package overview

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
)

type reader struct {
	active, peak   atomic.Int64
	discoveryError bool
}

func (c *reader) Discover(context.Context) (resource.Catalog, error) {
	if c.discoveryError {
		return resource.Catalog{}, errors.New("unavailable")
	}
	result := resource.Catalog{Warnings: []string{"extension unavailable"}}
	for i := range 12 {
		result.Resources = append(result.Resources, resource.Type{Version: "v1", Resource: fmt.Sprint(i), Verbs: []string{"list"}})
	}
	result.Resources = append(result.Resources, resource.Type{Version: "v2", Resource: "0", Verbs: []string{"list"}}, resource.Type{Resource: "pods/status", Verbs: []string{"list"}}, resource.Type{Resource: "create-only", Verbs: []string{"create"}})
	return result, nil
}
func (c *reader) CountResources(ctx context.Context, kind resource.Type) (int64, error) {
	current := c.active.Add(1)
	defer c.active.Add(-1)
	for peak := c.peak.Load(); current > peak && !c.peak.CompareAndSwap(peak, current); peak = c.peak.Load() {
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(time.Millisecond):
	}
	if kind.Resource == "1" {
		return 0, errors.New("forbidden")
	}
	return 0, nil
}
func (c *reader) OverviewNodes(context.Context) (Nodes, error) {
	return Nodes{}, errors.New("node forbidden")
}

type resolver struct {
	client    *reader
	requested string
}

func (r *resolver) OverviewClient(id string) (Client, error) {
	r.requested = id
	if id == "offline" {
		return nil, errors.New("disconnected")
	}
	return r.client, nil
}

func TestPartialOverviewKeepsUnknownSeparateFromZeroAndBoundsConcurrency(t *testing.T) {
	client := &reader{}
	res := &resolver{client: client}
	result, err := NewService(res).Snapshot(t.Context(), "cluster-two")
	if err != nil {
		t.Fatal(err)
	}
	if res.requested != "cluster-two" || result.ClusterID != "cluster-two" || result.UpdatedAt == "" || len(result.Resources) != 12 || len(result.Warnings) != 1 {
		t.Fatalf("unexpected overview: %+v", result)
	}
	if result.Nodes != nil || result.NodeError == "" || result.Resources[1].Count != nil || result.Resources[1].Error == "" || result.Resources[0].Count == nil || *result.Resources[0].Count != 0 {
		t.Fatal("unavailable value was represented as zero")
	}
	if client.peak.Load() > 4 || client.peak.Load() < 2 || client.active.Load() != 0 {
		t.Fatal("worker lifetime or concurrency incorrect")
	}
}
func TestOverviewErrorsAndCancellation(t *testing.T) {
	res := &resolver{client: &reader{discoveryError: true}}
	service := NewService(res)
	for _, id := range []string{"", "offline"} {
		if _, err := service.Snapshot(t.Context(), id); err == nil {
			t.Fatal("invalid target accepted")
		}
	}
	result, err := service.Snapshot(t.Context(), "one")
	if err != nil || len(result.Warnings) != 1 || len(result.Resources) != 0 {
		t.Fatal("discovery failure hid independent node result")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.Snapshot(ctx, "one"); err == nil {
		t.Fatal("canceled overview succeeded")
	}
}
