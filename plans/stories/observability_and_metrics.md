# Story: Observability & Metrics

## Goal
Provide Events, Status conditions, and Prometheus metrics to monitor propagation.

## Why
Operational transparency and SLA tracking (sections 5.6, 8, 9, 13, Traceability).

## Scope
- Events for create/update/skip/prune/errors.
- Status conditions and counters.
- Metrics endpoint with counters/gauges/histograms.

## Implementation
- Events: `src/adapters/events.py` helper emitting Kubernetes Events with reasons.
- Status: `src/agents/status.py` compute and update conditions and counts.
- Metrics: `src/adapters/metrics.py` using Prometheus client
  - `configpropagator_propagations_total`
  - `configpropagator_targets_gauge`
  - `configpropagator_updates_seconds` (histogram)
  - `configpropagator_errors_total`
  - `configpropagator_out_of_sync_gauge`
- Expose `/metrics` in `src/agents/operator.py` via HTTP server.

## Tests
- `tests/agents/test_status_conditions.py`: conditions transitions Ready/Progressing/Degraded.
- `tests/adapters/test_metrics_exposure.py`: metrics names exported; counters increment.
- `tests/adapters/test_events.py`: reasons emitted on key actions.

## Acceptance
- `kubectl describe` shows Events for propagate, skip, prune, error.
- Status lists synced/outOfSync and lastSyncTime.
- Metrics endpoint scrapes with expected series.
