package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"configpropagation/pkg/core"
)

func TestRecorderObserveReconcile(t *testing.T) {
	reg := prometheus.NewRegistry()
	rec := NewRecorder(reg)
	result := core.RolloutResult{
		TotalTargets:     5,
		CompletedTargets: []string{"a", "b", "c"},
		SkippedTargets:   []core.OutOfSyncItem{{Namespace: "ns"}},
	}
	rec.ObserveReconcile(result, 250*time.Millisecond)

	if got := testutil.ToFloat64(rec.propagations); got != 1 {
		t.Fatalf("expected propagations counter 1, got %f", got)
	}
	if got := testutil.ToFloat64(rec.targets); got != 5 {
		t.Fatalf("expected targets gauge 5, got %f", got)
	}
	if got := testutil.ToFloat64(rec.outOfSync); got != 1 {
		t.Fatalf("expected out of sync gauge 1, got %f", got)
	}
	if count := testutil.CollectAndCount(rec.updates); count != 1 {
		t.Fatalf("expected histogram observation, got %d", count)
	}
}

func TestRecorderObserveError(t *testing.T) {
	reg := prometheus.NewRegistry()
	rec := NewRecorder(reg)
	rec.ObserveError()
	rec.ObserveError()
	if got := testutil.ToFloat64(rec.errors); got != 2 {
		t.Fatalf("expected 2 errors, got %f", got)
	}
}
