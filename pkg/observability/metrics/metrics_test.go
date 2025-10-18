package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"configpropagation/pkg/core"
)

func TestRecordReconcileMetrics(t *testing.T) {
	ensureRegistered()
	propagationsTotal.Reset()
	targetsGauge.Reset()
	outOfSyncGauge.Set(0)

	baselineErrors := testutil.ToFloat64(errorsTotal)

	result := core.RolloutResult{TotalTargets: 4, CompletedCount: 3, OutOfSync: []core.OutOfSyncItem{{Namespace: "ns"}}}
	RecordReconcile(result, 2*time.Second, nil)

	if got := testutil.ToFloat64(propagationsTotal.WithLabelValues("success")); got != 1 {
		t.Fatalf("expected success counter 1, got %v", got)
	}
	if got := testutil.ToFloat64(targetsGauge.WithLabelValues("total")); got != 4 {
		t.Fatalf("expected total gauge 4, got %v", got)
	}
	if got := testutil.ToFloat64(targetsGauge.WithLabelValues("synced")); got != 3 {
		t.Fatalf("expected synced gauge 3, got %v", got)
	}
	if got := testutil.ToFloat64(outOfSyncGauge); got != 1 {
		t.Fatalf("expected out-of-sync gauge 1, got %v", got)
	}

	RecordReconcile(core.RolloutResult{}, time.Second, assertErr{})

	if got := testutil.ToFloat64(propagationsTotal.WithLabelValues("error")); got != 1 {
		t.Fatalf("expected error counter 1, got %v", got)
	}
	if got := testutil.ToFloat64(errorsTotal); got != baselineErrors+1 {
		t.Fatalf("expected errors total increment, got %v", got)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
