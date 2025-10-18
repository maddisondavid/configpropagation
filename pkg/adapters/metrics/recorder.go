package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"configpropagation/pkg/core"
)

// Recorder exposes helpers for recording Prometheus metrics about reconciliations.
type Recorder struct {
	propagations prometheus.Counter
	errors       prometheus.Counter
	targets      prometheus.Gauge
	outOfSync    prometheus.Gauge
	updates      prometheus.Histogram
}

// NewRecorder constructs a Recorder and registers the metrics with the provided registerer.
// If reg is nil the default Prometheus registerer is used.
func NewRecorder(reg prometheus.Registerer) *Recorder {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	r := &Recorder{
		propagations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "configpropagator_propagations_total",
			Help: "Total number of ConfigPropagation reconciliations.",
		}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "configpropagator_errors_total",
			Help: "Total number of reconciliation errors.",
		}),
		targets: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "configpropagator_targets_gauge",
			Help: "Number of target namespaces evaluated in the last reconcile.",
		}),
		outOfSync: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "configpropagator_out_of_sync_gauge",
			Help: "Number of target namespaces currently out of sync.",
		}),
		updates: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "configpropagator_updates_seconds",
			Help:    "Duration of successful reconciliation loops.",
			Buckets: prometheus.DefBuckets,
		}),
	}
	r.propagations = registerCounter(reg, r.propagations)
	r.errors = registerCounter(reg, r.errors)
	r.targets = registerGauge(reg, r.targets)
	r.outOfSync = registerGauge(reg, r.outOfSync)
	r.updates = registerHistogram(reg, r.updates)
	return r
}

// ObserveReconcile records a successful reconciliation with its duration.
func (r *Recorder) ObserveReconcile(result core.RolloutResult, duration time.Duration) {
	if r == nil {
		return
	}
	r.propagations.Inc()
	r.targets.Set(float64(result.TotalTargets))
	r.outOfSync.Set(float64(len(result.SkippedTargets)))
	r.updates.Observe(duration.Seconds())
}

// ObserveError increments the reconciliation error counter.
func (r *Recorder) ObserveError() {
	if r == nil {
		return
	}
	r.errors.Inc()
}

func registerCounter(reg prometheus.Registerer, c prometheus.Counter) prometheus.Counter {
	if err := reg.Register(c); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(prometheus.Counter); ok {
				return existing
			}
		}
		panic(err)
	}
	return c
}

func registerGauge(reg prometheus.Registerer, g prometheus.Gauge) prometheus.Gauge {
	if err := reg.Register(g); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(prometheus.Gauge); ok {
				return existing
			}
		}
		panic(err)
	}
	return g
}

func registerHistogram(reg prometheus.Registerer, h prometheus.Histogram) prometheus.Histogram {
	if err := reg.Register(h); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(prometheus.Histogram); ok {
				return existing
			}
		}
		panic(err)
	}
	return h
}
