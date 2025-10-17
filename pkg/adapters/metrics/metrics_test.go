package metrics

import (
        "testing"
        "time"

        "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/testutil"

        "configpropagation/pkg/agents/summary"
        "configpropagation/pkg/core"
)

func TestRecorderObserveReconcile(t *testing.T) {
        reg := prometheus.NewRegistry()
        rec := NewRecorder(reg)
        sum := &summary.Summary{Planned: []string{"a", "b"}, OutOfSync: []core.OutOfSyncItem{{Namespace: "b"}}}
        rec.ObserveReconcile(sum, nil, 2*time.Second)
        if got := testutil.ToFloat64(rec.propagations.WithLabelValues("success")); got != 1 {
                t.Fatalf("expected success counter 1, got %f", got)
        }
        if got := testutil.ToFloat64(rec.targets); got != 2 {
                t.Fatalf("expected targets gauge 2, got %f", got)
        }
        if got := testutil.ToFloat64(rec.outOfSync); got != 1 {
                t.Fatalf("expected out-of-sync 1, got %f", got)
        }

        rec.ObserveReconcile(nil, assertError{}, time.Second)
        if got := testutil.ToFloat64(rec.propagations.WithLabelValues("error")); got != 1 {
                t.Fatalf("expected error counter 1, got %f", got)
        }
        if got := testutil.ToFloat64(rec.errors); got != 1 {
                t.Fatalf("expected errors total 1, got %f", got)
        }
        if got := testutil.ToFloat64(rec.targets); got != 0 {
                t.Fatalf("expected targets reset to 0, got %f", got)
        }
}

type assertError struct{}

func (assertError) Error() string { return "boom" }

