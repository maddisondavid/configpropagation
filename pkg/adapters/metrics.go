package adapters

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	actionCreate = "create"
	actionUpdate = "update"
	actionSkip   = "skip"
	actionPrune  = "prune"
)

// MetricsRecorder captures Prometheus metrics for reconciliation activity.
type MetricsRecorder interface {
	// AddPropagations increments the propagation counter for the provided action.
	AddPropagations(action string, count int)
	// ObserveTargets records the most recent total and out-of-sync counts.
	ObserveTargets(total, outOfSync int)
	// ObserveReconcileDuration records the reconciliation duration.
	ObserveReconcileDuration(duration time.Duration)
	// IncError increments the error counter for the provided stage.
	IncError(stage string)
}

// NewNoopMetricsRecorder returns a MetricsRecorder that performs no-ops.
func NewNoopMetricsRecorder() MetricsRecorder {
	return noopMetricsRecorder{}
}

type noopMetricsRecorder struct{}

func (noopMetricsRecorder) AddPropagations(string, int)            {}
func (noopMetricsRecorder) ObserveTargets(int, int)                {}
func (noopMetricsRecorder) ObserveReconcileDuration(time.Duration) {}
func (noopMetricsRecorder) IncError(string)                        {}

type prometheusMetricsRecorder struct{}

var (
	propagationCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "configpropagator_propagations_total",
		Help: "Number of propagation actions by result.",
	}, []string{"action"})

	targetsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "configpropagator_targets_gauge",
		Help: "Latest number of target namespaces evaluated per reconcile.",
	})

	outOfSyncGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "configpropagator_out_of_sync_gauge",
		Help: "Latest number of target namespaces still pending sync.",
	})

	errorsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "configpropagator_errors_total",
		Help: "Total number of reconcile errors by stage.",
	}, []string{"stage"})

	reconcileHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "configpropagator_updates_seconds",
		Help:    "Histogram of reconciliation durations.",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	ctrlmetrics.Registry.MustRegister(propagationCounter, targetsGauge, outOfSyncGauge, errorsCounter, reconcileHistogram)
}

// NewPrometheusMetricsRecorder constructs a MetricsRecorder backed by Prometheus metrics.
func NewPrometheusMetricsRecorder() MetricsRecorder {
	return &prometheusMetricsRecorder{}
}

func (*prometheusMetricsRecorder) AddPropagations(action string, count int) {
	propagationCounter.WithLabelValues(action).Add(float64(count))
}

func (*prometheusMetricsRecorder) ObserveTargets(total, outOfSync int) {
	targetsGauge.Set(float64(total))
	outOfSyncGauge.Set(float64(outOfSync))
}

func (*prometheusMetricsRecorder) ObserveReconcileDuration(duration time.Duration) {
	reconcileHistogram.Observe(duration.Seconds())
}

func (*prometheusMetricsRecorder) IncError(stage string) {
	errorsCounter.WithLabelValues(stage).Inc()
}

// Action constants exported for reuse in controllers.
const (
	MetricsActionCreate = actionCreate
	MetricsActionUpdate = actionUpdate
	MetricsActionSkip   = actionSkip
	MetricsActionPrune  = actionPrune
)
