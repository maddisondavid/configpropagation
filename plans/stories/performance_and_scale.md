# Story: Performance & Scale

## Goal
Meet propagation SLA (95% updates ≤60s in-cluster) and scale to thousands of target Namespaces.

## Why
Ensures service levels and operational efficiency (sections 8, 15).

## Scope
- Efficient informers/caches; avoid N×M loops.
- Batching and backpressure tuning.
- Load/perf testing plan and tunables.

## Implementation
- Caching: index Namespaces by labels; cache source ConfigMaps by key; avoid redundant API calls.
- Batching: group patch requests when safe; tune worker concurrency.
- Config: tune via env/flags `WORKERS`, `BATCH_SIZE`, `RESYNC_SECONDS`.
- Perf harness: `tests/perf/` synthetic Namespaces and ConfigMap sizes; measure p95.
- Document knobs in `docs/performance.md`.

## Tests
- `tests/perf/test_scale_thousands.py` (marked slow): simulate 1k namespaces; verify p95 under threshold locally.

## Acceptance
- Under synthetic load, p95 end-to-end ≤ 60s with documented environment sizing.
- CPU/memory steady and acceptable; backpressure visible in metrics.
