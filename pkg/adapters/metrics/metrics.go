package metrics

import (
        "time"

        "github.com/prometheus/client_golang/prometheus"
        ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

        "configpropagation/pkg/agents/summary"
)

// Recorder provides helpers for emitting Prometheus metrics.
type Recorder struct {
        propagations *prometheus.CounterVec
        targets      prometheus.Gauge
        updates      prometheus.Histogram
        errors       prometheus.Counter
        outOfSync    prometheus.Gauge
}

var defaultRecorder = newRecorder(ctrlmetrics.Registry)

// Default returns the shared metrics recorder registered with controller-runtime.
func Default() *Recorder { return defaultRecorder }

// NewRecorder creates a Recorder bound to the provided registry.
func NewRecorder(reg prometheus.Registerer) *Recorder { return newRecorder(reg) }

func newRecorder(reg prometheus.Registerer) *Recorder {
        r := &Recorder{
                propagations: prometheus.NewCounterVec(prometheus.CounterOpts{
                        Name: "configpropagator_propagations_total",
                        Help: "Total number of ConfigPropagation reconciliations partitioned by outcome.",
                }, []string{"status"}),
                targets: prometheus.NewGauge(prometheus.GaugeOpts{
                        Name: "configpropagator_targets_gauge",
                        Help: "Current number of target namespaces planned for propagation.",
                }),
                updates: prometheus.NewHistogram(prometheus.HistogramOpts{
                        Name:    "configpropagator_updates_seconds",
                        Help:    "Histogram of reconciliation durations in seconds.",
                        Buckets: prometheus.DefBuckets,
                }),
                errors: prometheus.NewCounter(prometheus.CounterOpts{
                        Name: "configpropagator_errors_total",
                        Help: "Total number of reconciliation errors.",
                }),
                outOfSync: prometheus.NewGauge(prometheus.GaugeOpts{
                        Name: "configpropagator_out_of_sync_gauge",
                        Help: "Number of namespaces currently marked out-of-sync.",
                }),
        }
        if reg != nil {
                reg.MustRegister(r.propagations, r.targets, r.updates, r.errors, r.outOfSync)
        }
        return r
}

// ObserveReconcile records metrics for a reconciliation attempt.
func (r *Recorder) ObserveReconcile(sum *summary.Summary, reconcileErr error, duration time.Duration) {
        if r == nil {
                return
        }
        status := "success"
        if reconcileErr != nil {
                status = "error"
                r.errors.Inc()
        }
        r.propagations.WithLabelValues(status).Inc()
        if sum == nil {
                r.targets.Set(0)
                r.outOfSync.Set(0)
        } else {
                r.targets.Set(float64(len(sum.Planned)))
                r.outOfSync.Set(float64(sum.OutOfSyncCount()))
        }
        r.updates.Observe(duration.Seconds())
}

