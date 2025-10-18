package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"configpropagation/pkg/core"
)

var (
	registerOnce sync.Once

	propagationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "configpropagator_propagations_total",
		Help: "Total number of ConfigPropagation reconciliations grouped by result.",
	}, []string{"result"})

	targetsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "configpropagator_targets_gauge",
		Help: "Number of ConfigMap targets observed during reconciliation grouped by state.",
	}, []string{"state"})

	updatesHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "configpropagator_updates_seconds",
		Help:    "Histogram of reconciliation duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
	})

	errorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "configpropagator_errors_total",
		Help: "Total number of reconciliation errors.",
	})

	outOfSyncGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "configpropagator_out_of_sync_gauge",
		Help: "Number of namespaces that remain out of sync after reconciliation.",
	})
)

func ensureRegistered() {
	registerOnce.Do(func() {
		ctrlmetrics.Registry.MustRegister(propagationsTotal, targetsGauge, updatesHistogram, errorsTotal, outOfSyncGauge)
	})
}

// RecordReconcile updates the metrics based on the reconciliation result.
func RecordReconcile(result core.RolloutResult, duration time.Duration, reconcileErr error) {
	ensureRegistered()

	outcome := "success"
	if reconcileErr != nil {
		outcome = "error"
		errorsTotal.Inc()
	}

	propagationsTotal.WithLabelValues(outcome).Inc()
	updatesHistogram.Observe(duration.Seconds())

	targetsGauge.WithLabelValues("total").Set(float64(result.TotalTargets))
	targetsGauge.WithLabelValues("synced").Set(float64(result.CompletedCount))

	outOfSync := result.TotalTargets - result.CompletedCount
	if outOfSync < 0 {
		outOfSync = 0
	}
	if len(result.OutOfSync) > outOfSync {
		outOfSync = len(result.OutOfSync)
	}
	outOfSyncGauge.Set(float64(outOfSync))
}
