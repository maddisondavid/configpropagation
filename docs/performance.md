# Performance & Scalability Guide

This guide documents expectations, tuning knobs, and validation steps to meet the SLA: 95% of target updates applied within 60s for in-cluster propagation.

## Targets & Metrics
- SLA: 95th percentile end-to-end propagation time ≤ 60s
- Scale: thousands of target namespaces per propagation
- Metrics:
  - `configpropagator_updates_seconds` (histogram): per-target update duration
  - `configpropagator_targets_gauge`: targets per propagation
  - `configpropagator_errors_total`: failures by reason
  - `configpropagator_out_of_sync_gauge`: out-of-sync targets

## Tuning Knobs
- Batch size: `strategy.batchSize` (CR) or `BATCH_SIZE` (env default) — rolling updates per reconcile iteration (default 5)
- Workers: `WORKERS` — parallel target workers per controller instance (default 4–8)
- Resync: `RESYNC_SECONDS` — periodic resync tick (default 30–60)
- Rate limit: `RATE_LIMIT_QPS`/`BURST` for client calls (defaults match k8s client best practices)
- Backoff: `RETRY_BASE_MS`, `RETRY_MAX_MS` — exponential backoff bounds

## Recommended Defaults
- Small blast radius: batchSize=5, workers=8, resync=45s
- Moderate clusters (≤500 namespaces): workers=8–12, QPS/Burst 20/40
- Large clusters (1k–5k namespaces): workers=16–24, QPS/Burst 40/80; consider multiple replicas

## Design Notes
- Uses shared informers and label indexers to avoid N×M API calls
- Hash-based drift detection prevents unnecessary writes
- Rolling strategy limits concurrent mutations; immediate permitted when needed
- Partial failures isolated; retries with backoff

## Validation Procedure
1) Prepare test namespaces (e.g., 1k) labeled to match a propagation
2) Create a `ConfigPropagation` pointing to a medium-sized ConfigMap (~64KB total data)
3) Start metric scraping (Prometheus) and record baseline
4) Update the source ConfigMap (single key change)
5) Measure `configpropagator_updates_seconds` p95 and total duration until `syncedCount == targetCount`
6) Adjust batch size/workers and repeat to find optimal settings

## Payload Size Considerations
- Kubernetes ConfigMap total size limit ~1MB (object size). Keep effective payload well below (e.g., ≤ 256KB) to avoid fragmentation and API pressure
- The operator warns when approaching limits and can block when exceeding

## Troubleshooting
- High p95: increase workers and/or batch size; verify API server QPS/Burst; check RBAC denials slowing retries
- Persistent out-of-sync: inspect Events; confirm conflict policy; check network/API errors
- Client throttling: increase client QPS/Burst cautiously; observe API server saturation

## Capacity Planning
- Estimate update time ~ (targets / (workers * batchSize)) * avg_per_target_seconds
- Keep controller CPU < 70% and memory < 60% under peak; scale replicas horizontally if needed

## References
- Capability spec sections 5, 8, 9; Risk Register section 10; Dependencies section 11
