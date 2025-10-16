# Story: Rollout Strategies

## Goal
Support `immediate` and `rolling` updates with configurable `batchSize` (default 5) to limit blast radius.

## Why
Provides controlled rollout and SLA compliance (sections 5.3, 8, 10).

## Scope
- Strategy abstraction; batch scheduling; progress tracking.

## Implementation
- Strategy interface: `src/core/rollout.py`
  - `plan_targets(all_targets, status, batch_size)` -> list of namespaces to update now.
  - Track per-CR progress in memory (and in status when useful) to continue next batches.
- Reconciler integration: only process upto batch per tick when `rolling`.
- Config: read defaults from CRD/webhook; override from spec.

## Tests
- `tests/core/test_rollout.py`: immediate updates all; rolling respects batchSize and progresses across ticks.

## Acceptance
- Immediate updates schedule all matching Namespaces in one loop.
- Rolling schedules up to `batchSize` per iteration, then advances.
